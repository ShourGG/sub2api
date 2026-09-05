package service

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

var openAIResponsesRejectedInputFieldPattern = regexp.MustCompile(`(?i)^input(?:\[(\d+)\]|\.(\d+))\.(.+)$`)

type openAIResponsesRejectedInputMetadata struct {
	Found          bool
	ItemType       string
	FieldNames     []string
	IDState        string
	CallIDState    string
	RoleState      string
	ContentState   string
	ContentElement string
}

type openAIResponsesRejectedIDItemMetadata struct {
	Found      bool
	ItemType   string
	IDState    string
	IDLength   int
	IDPrefix   string
	FieldNames []string
}

func openAIResponsesRejectedIDItemMetadataAt(body []byte, index int) openAIResponsesRejectedIDItemMetadata {
	meta := openAIResponsesRejectedIDItemMetadata{IDState: "missing", IDPrefix: "none"}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return meta
	}
	items := input.Array()
	if index < 0 || index >= len(items) || !items[index].IsObject() {
		return meta
	}

	meta.Found = true
	fields := items[index].Map()
	meta.ItemType = strings.TrimSpace(fields["type"].String())
	meta.FieldNames = make([]string, 0, len(fields))
	for name := range fields {
		meta.FieldNames = append(meta.FieldNames, name)
	}
	sort.Strings(meta.FieldNames)

	id, exists := fields["id"]
	if !exists {
		return meta
	}
	switch id.Type {
	case gjson.Null:
		meta.IDState = "null"
	case gjson.String:
		value := id.String()
		meta.IDState = "string"
		meta.IDLength = len(value)
		meta.IDPrefix = classifyOpenAIResponsesRejectedIDPrefix(value)
	default:
		meta.IDState = strings.ToLower(id.Type.String())
	}
	return meta
}

func openAIResponsesRejectedInputMetadataAt(body []byte, index int) openAIResponsesRejectedInputMetadata {
	meta := openAIResponsesRejectedInputMetadata{
		IDState:      "missing",
		CallIDState:  "missing",
		RoleState:    "missing",
		ContentState: "missing",
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return meta
	}
	items := input.Array()
	if index < 0 || index >= len(items) || !items[index].IsObject() {
		return meta
	}
	item := items[index]
	meta.Found = true
	meta.ItemType = strings.TrimSpace(item.Get("type").String())
	fields := item.Map()
	meta.FieldNames = make([]string, 0, len(fields))
	for name := range fields {
		meta.FieldNames = append(meta.FieldNames, name)
	}
	sort.Strings(meta.FieldNames)
	meta.IDState = classifyOpenAIResponsesJSONField(item.Get("id"))
	meta.CallIDState = classifyOpenAIResponsesJSONField(item.Get("call_id"))
	meta.RoleState = classifyOpenAIResponsesJSONField(item.Get("role"))
	content := item.Get("content")
	meta.ContentState = classifyOpenAIResponsesJSONField(content)
	if content.IsArray() {
		for elementIndex, element := range content.Array() {
			if elementIndex == 0 {
				meta.ContentElement = strings.ToLower(strings.TrimSpace(element.Type.String()))
				break
			}
		}
	}
	return meta
}

func classifyOpenAIResponsesJSONField(value gjson.Result) string {
	if !value.Exists() {
		return "missing"
	}
	switch value.Type {
	case gjson.Null:
		return "null"
	case gjson.String:
		if strings.TrimSpace(value.String()) == "" {
			return "empty_string"
		}
		return "string"
	case gjson.Number:
		return "number"
	case gjson.True, gjson.False:
		return "boolean"
	case gjson.JSON:
		return "object"
	default:
		return strings.ToLower(strings.TrimSpace(value.Type.String()))
	}
}

func openAIResponsesRejectedInputFieldFromResponse(responseBody []byte) (string, int, string, bool) {
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))
	if code != "invalid_parameter" {
		return "", 0, "", false
	}
	param := strings.TrimSpace(gjson.GetBytes(responseBody, "error.param").String())
	if param == "" {
		inner := strings.TrimSpace(gjson.GetBytes(responseBody, "error.message").String())
		if strings.HasPrefix(inner, "{") {
			param = strings.TrimSpace(gjson.Get(inner, "error.param").String())
			if param == "" {
				if lastBrace := strings.LastIndex(inner, "}"); lastBrace >= 0 {
					param = strings.TrimSpace(gjson.Get(inner[:lastBrace+1], "error.param").String())
				}
			}
		}
	}
	match := openAIResponsesRejectedInputFieldPattern.FindStringSubmatch(param)
	if len(match) != 4 {
		return "", 0, "", false
	}
	rawIndex := match[1]
	if rawIndex == "" {
		rawIndex = match[2]
	}
	index := 0
	for _, ch := range rawIndex {
		if ch < '0' || ch > '9' {
			return "", 0, "", false
		}
		index = index*10 + int(ch-'0')
	}
	return param, index, strings.TrimSpace(match[3]), true
}

func logOpenAIResponsesRejectedFieldDiagnostic(ctx context.Context, account *Account, upstreamReq *http.Request, retryBody, responseBody []byte, route string) {
	param, index, subpath, ok := openAIResponsesRejectedInputFieldFromResponse(responseBody)
	if !ok {
		return
	}
	wireBody, wireBodySource := snapshotOpenAIUpstreamRequestBody(upstreamReq, retryBody)
	wireMeta := openAIResponsesRejectedInputMetadataAt(wireBody, index)
	retryMeta := openAIResponsesRejectedInputMetadataAt(retryBody, index)
	accountID := int64(0)
	if account != nil {
		accountID = account.ID
	}
	logger.FromContext(ctx).Warn("openai.responses_rejected_input_diagnostic",
		zap.Int64("account_id", accountID),
		zap.String("route", route),
		zap.String("rejected_param", param),
		zap.Int("rejected_index", index),
		zap.String("rejected_subpath", subpath),
		zap.String("wire_body_source", wireBodySource),
		zap.Int("wire_body_bytes", len(wireBody)),
		zap.Bool("wire_item_found", wireMeta.Found),
		zap.String("wire_item_type", wireMeta.ItemType),
		zap.String("wire_id_state", wireMeta.IDState),
		zap.String("wire_call_id_state", wireMeta.CallIDState),
		zap.String("wire_role_state", wireMeta.RoleState),
		zap.String("wire_content_state", wireMeta.ContentState),
		zap.String("wire_content_element_type", wireMeta.ContentElement),
		zap.Strings("wire_field_names", wireMeta.FieldNames),
		zap.Int("retry_body_bytes", len(retryBody)),
		zap.Bool("retry_item_found", retryMeta.Found),
		zap.String("retry_item_type", retryMeta.ItemType),
		zap.String("retry_id_state", retryMeta.IDState),
		zap.String("retry_call_id_state", retryMeta.CallIDState),
		zap.String("retry_role_state", retryMeta.RoleState),
		zap.String("retry_content_state", retryMeta.ContentState),
		zap.String("retry_content_element_type", retryMeta.ContentElement),
		zap.Strings("retry_field_names", retryMeta.FieldNames),
	)
}

func classifyOpenAIResponsesRejectedIDPrefix(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "empty"
	}
	for _, prefix := range []string{"msg", "item", "fc", "ctc", "rs", "ws", "tsc", "ctco"} {
		if id == prefix || strings.HasPrefix(id, prefix+"_") {
			return prefix
		}
	}
	return "other"
}

func openAIResponsesRejectedIDIndexFromResponse(responseBody []byte) (int, bool) {
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))
	message := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	param := strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "error.param").String()))
	if code != "invalid_parameter" {
		return 0, false
	}
	paramIndex, paramOK := openAIResponsesRejectedIDIndex(param)
	messageIndex, messageOK := openAIResponsesInvalidValueIDIndexFromMessage(message)
	return paramIndex, paramOK && messageOK && paramIndex == messageIndex
}

func snapshotOpenAIUpstreamRequestBody(req *http.Request, fallback []byte) ([]byte, string) {
	if req == nil || req.GetBody == nil {
		return fallback, "retry_body_fallback"
	}
	body, err := req.GetBody()
	if err != nil {
		return fallback, "retry_body_get_failed"
	}
	defer func() { _ = body.Close() }()
	wireBody, err := io.ReadAll(body)
	if err != nil || len(wireBody) == 0 {
		return fallback, "retry_body_read_failed"
	}
	return wireBody, "request_get_body"
}

func logOpenAIResponsesRejectedIDDiagnostic(ctx context.Context, account *Account, upstreamReq *http.Request, retryBody, responseBody []byte, route string) {
	index, ok := openAIResponsesRejectedIDIndexFromResponse(responseBody)
	if !ok {
		return
	}
	wireBody, wireBodySource := snapshotOpenAIUpstreamRequestBody(upstreamReq, retryBody)
	wireMeta := openAIResponsesRejectedIDItemMetadataAt(wireBody, index)
	retryMeta := openAIResponsesRejectedIDItemMetadataAt(retryBody, index)
	accountID := int64(0)
	if account != nil {
		accountID = account.ID
	}
	logger.FromContext(ctx).Warn("openai.responses_rejected_input_id_diagnostic",
		zap.Int64("account_id", accountID),
		zap.String("route", route),
		zap.Int("rejected_index", index),
		zap.String("wire_body_source", wireBodySource),
		zap.Int("wire_body_bytes", len(wireBody)),
		zap.Bool("wire_item_found", wireMeta.Found),
		zap.String("wire_item_type", wireMeta.ItemType),
		zap.String("wire_id_state", wireMeta.IDState),
		zap.Int("wire_id_length", wireMeta.IDLength),
		zap.String("wire_id_prefix", wireMeta.IDPrefix),
		zap.Strings("wire_field_names", wireMeta.FieldNames),
		zap.Int("retry_body_bytes", len(retryBody)),
		zap.Bool("retry_item_found", retryMeta.Found),
		zap.String("retry_item_type", retryMeta.ItemType),
		zap.String("retry_id_state", retryMeta.IDState),
		zap.Int("retry_id_length", retryMeta.IDLength),
		zap.String("retry_id_prefix", retryMeta.IDPrefix),
		zap.Strings("retry_field_names", retryMeta.FieldNames),
	)
}
