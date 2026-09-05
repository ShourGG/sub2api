package service

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const maxOpenAIResponsesRejectedFieldRetries = 6

var (
	openAIResponsesRejectedNamespaceParamPattern           = regexp.MustCompile(`(?i)^input(?:\[(\d+)\]|\.(\d+))\.namespace$`)
	openAIResponsesRejectedStatusParamPattern              = regexp.MustCompile(`(?i)^input(?:\[(\d+)\]|\.(\d+))\.status$`)
	openAIResponsesRejectedContentParamPattern             = regexp.MustCompile(`(?i)^input(?:\[(\d+)\]|\.(\d+))\.content$`)
	openAIResponsesRejectedCacheParamPattern               = regexp.MustCompile(`(?i)^input(?:\[(\d+)\]|\.(\d+))\.prompt_cache_breakpoint$`)
	openAIResponsesRejectedIDParamPattern                  = regexp.MustCompile(`(?i)^input(?:\[(\d+)\]|\.(\d+))\.id$`)
	openAIResponsesInvalidValueIDMessagePattern            = regexp.MustCompile(`(?i)^invalid\s+value\s+for\s+["']?(input(?:\[\d+\]|\.\d+)\.id)["']?\s*:\s*expected\s+a\s+value\.?$`)
	openAIResponsesRejectedCallIDParamPattern              = regexp.MustCompile(`(?i)^input(?:\[(\d+)\]|\.(\d+))\.call_id$`)
	openAIResponsesInvalidValueCallIDMessagePattern        = regexp.MustCompile(`(?i)^invalid\s+value\s+for\s+["']?(input(?:\[\d+\]|\.\d+)\.call_id)["']?\s*:\s*expected\s+a\s+value\.?$`)
	openAIResponsesRejectedRoleParamPattern                = regexp.MustCompile(`(?i)^input(?:\[(\d+)\]|\.(\d+))\.role$`)
	openAIResponsesInvalidValueRoleMessagePattern          = regexp.MustCompile(`(?i)^invalid\s+value\s+for\s+["']?(input(?:\[\d+\]|\.\d+)\.role)["']?\s*:\s*expected\s+a\s+value\.?$`)
	openAIResponsesRejectedInputToolsParamPattern          = regexp.MustCompile(`(?i)^input(?:\[(\d+)\]|\.(\d+))\.tools$`)
	openAIResponsesRejectedContentElementParamPattern      = regexp.MustCompile(`(?i)^input(?:\[(\d+)\]|\.(\d+))\.content\.(\d+)$`)
	openAIResponsesInvalidValueInputToolsMessagePattern    = regexp.MustCompile(`(?i)^invalid\s+value\s+for\s+["']?(input(?:\[\d+\]|\.\d+)\.tools)["']?\s*\.?$`)
	openAIResponsesRejectedMessageParamPattern             = regexp.MustCompile(`(?i)(?:unknown|unsupported)[ _-]+parameter\s*(?::|=|is)?\s*["']?(max_output_tokens|truncation|input(?:\[\d+\]|\.\d+)\.(?:namespace|status))(?:["']|\b)`)
	openAIResponsesInvalidTypeMessageParamPattern          = regexp.MustCompile(`(?i)invalid[ _-]+type\s+for\s+["']?(input(?:\[\d+\]|\.\d+)\.content)(?:["']|\b)[^\n]*\b(?:got|received)\s+null\b`)
	openAIResponsesInvalidTypeContentElementMessagePattern = regexp.MustCompile(`(?i)^invalid\s+value\s+for\s+["']?(input(?:\[\d+\]|\.\d+)\.content\.\d+)["']?\s*:\s*expected\s+an\s+object,?\s+but\s+got\s+null\s+instead\.?$`)
	openAIResponsesMaxZeroContentMessagePattern            = regexp.MustCompile(`(?i)invalid\s+["']?(input(?:\[\d+\]|\.\d+)\.content)["']?\s*:\s*array too long\.[^\n]*maximum length 0\b`)
	openAIResponsesCacheModelRejectionPattern              = regexp.MustCompile(`(?i)["']?(prompt_cache_breakpoint|input(?:\[\d+\]|\.\d+)\.prompt_cache_breakpoint)["']?\s+is\s+not\s+supported\s+on\s+this\s+model\b`)
	openAIResponsesToolParametersParamPattern              = regexp.MustCompile(`(?i)^(?:tools|input)\[\d+\](?:\.tools\[\d+\])*(?:\.function)?\.parameters$`)
	openAIResponsesMissingSchemaTypePattern                = regexp.MustCompile(`(?i)\bgot\s+["']?type\s*:\s*["']?none["']?`)
)

type openAIResponsesRejectedFieldRetryState struct {
	mu             sync.Mutex
	budget         *openAIResponsesRejectedFieldRetryBudget
	seenBodyHashes map[[sha256.Size]byte]struct{}
}

type openAIResponsesRejectedFieldRetryBudget struct {
	mu       sync.Mutex
	attempts int
}

const openAIResponsesRejectedFieldRetryBudgetContextKey = "openai_responses_rejected_field_retry_budget"

// ResetOpenAIResponsesRejectedFieldRetryBudget starts a fresh bounded
// compatibility-repair window for a same-account upstream retry. The handler
// uses this only after a provider 429, where the next attempt is a new upstream
// request but must still be able to repair the original inbound body.
func ResetOpenAIResponsesRejectedFieldRetryBudget(c *gin.Context) {
	if c == nil {
		return
	}
	value, exists := c.Get(openAIResponsesRejectedFieldRetryBudgetContextKey)
	if !exists {
		return
	}
	budget, ok := value.(*openAIResponsesRejectedFieldRetryBudget)
	if !ok || budget == nil {
		return
	}
	budget.mu.Lock()
	budget.attempts = 0
	budget.mu.Unlock()
}

// openAIResponsesRejectedFieldRetryStateForRequest returns a fresh loop guard
// for one account attempt backed by the inbound request's shared retry budget.
// A later account may apply the same compatibility transform, while all account
// attempts together remain bounded.
func openAIResponsesRejectedFieldRetryStateForRequest(c *gin.Context, initialBody []byte) *openAIResponsesRejectedFieldRetryState {
	var budget *openAIResponsesRejectedFieldRetryBudget
	if c != nil {
		if existing, ok := c.Get(openAIResponsesRejectedFieldRetryBudgetContextKey); ok {
			budget, _ = existing.(*openAIResponsesRejectedFieldRetryBudget)
		}
	}
	if budget == nil {
		budget = &openAIResponsesRejectedFieldRetryBudget{}
		if c != nil {
			c.Set(openAIResponsesRejectedFieldRetryBudgetContextKey, budget)
		}
	}
	return newOpenAIResponsesRejectedFieldRetryStateWithBudget(initialBody, budget)
}

func newOpenAIResponsesRejectedFieldRetryState(initialBody []byte) *openAIResponsesRejectedFieldRetryState {
	return newOpenAIResponsesRejectedFieldRetryStateWithBudget(initialBody, &openAIResponsesRejectedFieldRetryBudget{})
}

func newOpenAIResponsesRejectedFieldRetryStateWithBudget(initialBody []byte, budget *openAIResponsesRejectedFieldRetryBudget) *openAIResponsesRejectedFieldRetryState {
	state := &openAIResponsesRejectedFieldRetryState{
		budget:         budget,
		seenBodyHashes: make(map[[sha256.Size]byte]struct{}, maxOpenAIResponsesRejectedFieldRetries+1),
	}
	state.remember(initialBody)
	return state
}

func (s *openAIResponsesRejectedFieldRetryState) Allow(nextBody []byte) bool {
	if s == nil || s.budget == nil || len(nextBody) == 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bodyHash := sha256.Sum256(nextBody)
	if _, seen := s.seenBodyHashes[bodyHash]; seen {
		return false
	}
	s.budget.mu.Lock()
	defer s.budget.mu.Unlock()
	if s.budget.attempts >= maxOpenAIResponsesRejectedFieldRetries {
		return false
	}
	s.seenBodyHashes[bodyHash] = struct{}{}
	s.budget.attempts++
	return true
}

func (s *openAIResponsesRejectedFieldRetryState) remember(body []byte) {
	if s == nil || len(body) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rememberLocked(body)
}

func (s *openAIResponsesRejectedFieldRetryState) rememberLocked(body []byte) {
	if s.seenBodyHashes == nil {
		s.seenBodyHashes = make(map[[sha256.Size]byte]struct{}, maxOpenAIResponsesRejectedFieldRetries+1)
	}
	s.seenBodyHashes[sha256.Sum256(body)] = struct{}{}
}

func normalizeOpenAIResponsesRejectedFieldRetryBody(statusCode int, body, responseBody []byte) ([]byte, string, bool, error) {
	if statusCode != http.StatusBadRequest || len(body) == 0 || len(responseBody) == 0 {
		return nil, "", false, nil
	}

	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))
	message := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	param := strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "error.param").String()))
	if param == "" {
		// A few gateways wrap the complete upstream error JSON inside
		// error.message and omit the outer param. Recover only that exact nested
		// param; do not infer a target from free-form text.
		inner := strings.TrimSpace(gjson.GetBytes(responseBody, "error.message").String())
		if strings.HasPrefix(inner, "{") {
			param = strings.ToLower(strings.TrimSpace(gjson.Get(inner, "error.param").String()))
			if param == "" {
				if lastBrace := strings.LastIndex(inner, "}"); lastBrace >= 0 {
					param = strings.ToLower(strings.TrimSpace(gjson.Get(inner[:lastBrace+1], "error.param").String()))
				}
			}
		}
	}
	if code == "invalid_parameter" {
		paramIndex, paramOK := openAIResponsesRejectedIDIndex(param)
		messageIndex, messageOK := openAIResponsesInvalidValueIDIndexFromMessage(message)
		if indexedRejectedFieldMatches(param, paramIndex, paramOK, messageIndex, messageOK) {
			return removeOpenAIResponsesRejectedIDAtIndex(body, paramIndex)
		}
		paramIndex, paramOK = openAIResponsesRejectedCallIDIndex(param)
		messageIndex, messageOK = openAIResponsesInvalidValueCallIDIndexFromMessage(message)
		if indexedRejectedFieldMatches(param, paramIndex, paramOK, messageIndex, messageOK) {
			return removeOpenAIResponsesMissingCallIDItems(body, paramIndex)
		}
		paramIndex, paramOK = openAIResponsesRejectedRoleIndex(param)
		messageIndex, messageOK = openAIResponsesInvalidValueRoleIndexFromMessage(message)
		if indexedRejectedFieldMatches(param, paramIndex, paramOK, messageIndex, messageOK) {
			return removeOpenAIResponsesMissingRoleItems(body, paramIndex)
		}
		paramIndex, paramOK = openAIResponsesRejectedInputToolsIndex(param)
		messageIndex, messageOK = openAIResponsesInvalidValueInputToolsIndexFromMessage(message)
		if indexedRejectedFieldMatches(param, paramIndex, paramOK, messageIndex, messageOK) {
			return removeOpenAIResponsesRejectedInputToolsAtIndex(body, paramIndex)
		}
		contentItemIndex, contentElementIndex, contentParamOK := openAIResponsesRejectedContentElementIndices(param)
		messageItemIndex, messageElementIndex, contentMessageOK := openAIResponsesInvalidValueContentElementIndicesFromMessage(message)
		if contentParamOK && contentMessageOK && contentItemIndex == messageItemIndex && contentElementIndex == messageElementIndex {
			return removeOpenAIResponsesRejectedNullContentElementAtIndex(body, contentItemIndex, contentElementIndex)
		}
	}
	if code == "invalid_function_parameters" &&
		openAIResponsesToolParametersParamPattern.MatchString(param) &&
		openAIResponsesMissingSchemaTypePattern.MatchString(message) {
		retryBody, changed, err := sanitizeOpenAIResponsesToolParameterTypes(body)
		if err != nil {
			return nil, "", false, fmt.Errorf("repair rejected tool parameter root type: %w", err)
		}
		if changed {
			return retryBody, "tool parameter root type rejection", true, nil
		}
	}
	cacheMessageParam := openAIResponsesCacheModelRejectionParamFromMessage(message)
	cacheParam := param
	if cacheParam == "" {
		cacheParam = cacheMessageParam
	}
	cacheParamMatchesMessage := cacheMessageParam == "" || cacheParam == cacheMessageParam
	cacheModelRejection := code == "invalid_parameter" || cacheMessageParam != ""
	if cacheParam != "" && cacheParamMatchesMessage && cacheModelRejection {
		if cacheParam == "prompt_cache_breakpoint" && gjson.GetBytes(body, cacheParam).Exists() {
			retryBody, err := sjson.DeleteBytes(body, cacheParam)
			if err != nil {
				return nil, "", false, fmt.Errorf("delete rejected prompt_cache_breakpoint: %w", err)
			}
			return retryBody, "prompt_cache_breakpoint parameter rejection", true, nil
		}
		if index, ok := openAIResponsesRejectedCacheIndex(cacheParam); ok {
			return removeOpenAIResponsesRejectedCacheAtIndex(body, index)
		}
	}
	if isExplicitOpenAIResponsesFieldRejection(code, message) {
		messageParam := openAIResponsesRejectedParamFromMessage(message)
		if param != "" && messageParam != "" && param != messageParam {
			return nil, "", false, nil
		}
		if param == "" {
			param = messageParam
		}
		if index, ok := openAIResponsesRejectedNamespaceIndex(param); ok {
			return removeOpenAIResponsesRejectedNamespaceAtIndex(body, index)
		}
		if index, ok := openAIResponsesRejectedStatusIndex(param); ok {
			return removeOpenAIResponsesRejectedStatusAtIndex(body, index)
		}
		if param == "max_output_tokens" && gjson.GetBytes(body, "max_output_tokens").Exists() {
			retryBody, err := sjson.DeleteBytes(body, "max_output_tokens")
			if err != nil {
				return nil, "", false, fmt.Errorf("delete rejected max_output_tokens: %w", err)
			}
			return retryBody, "max_output_tokens parameter rejection", true, nil
		}
		if param == "truncation" && gjson.GetBytes(body, "truncation").Exists() {
			retryBody, err := sjson.DeleteBytes(body, "truncation")
			if err != nil {
				return nil, "", false, fmt.Errorf("delete rejected truncation: %w", err)
			}
			return retryBody, "truncation parameter rejection", true, nil
		}
	}

	messageContentParam := openAIResponsesInvalidTypeParamFromMessage(message)
	contentParam := param
	if contentParam == "" {
		contentParam = messageContentParam
	}
	if index, ok := openAIResponsesRejectedContentIndex(contentParam); ok &&
		contentParam == messageContentParam && isExplicitOpenAIResponsesNullContentRejection(code, message) {
		return normalizeOpenAIResponsesRejectedNullContentAtIndex(body, index)
	}
	maxZeroContentParam := openAIResponsesMaxZeroContentParamFromMessage(message)
	if index, ok := openAIResponsesRejectedContentIndex(param); ok &&
		param == maxZeroContentParam && code == "array_above_max_length" {
		return removeOpenAIResponsesRejectedReasoningContentAtIndex(body, index)
	}
	return nil, "", false, nil
}

// Require both the structured param and message to identify the same indexed
// field. The caller may recover param from a nested error.message wrapper, but
// free-form message text alone is never enough to trigger a mutation.
func indexedRejectedFieldMatches(param string, paramIndex int, paramOK bool, messageIndex int, messageOK bool) bool {
	return strings.TrimSpace(param) != "" && paramOK && messageOK && paramIndex == messageIndex
}

func isExplicitOpenAIResponsesFieldRejection(code, message string) bool {
	switch strings.TrimSpace(code) {
	case "unknown_parameter", "unsupported_parameter":
		return true
	}
	return strings.Contains(message, "unknown parameter") ||
		strings.Contains(message, "unsupported parameter")
}

func openAIResponsesRejectedParamFromMessage(message string) string {
	match := openAIResponsesRejectedMessageParamPattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(match[1]))
}

func openAIResponsesMaxZeroContentParamFromMessage(message string) string {
	match := openAIResponsesMaxZeroContentMessagePattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(match[1]))
}

func openAIResponsesInvalidTypeParamFromMessage(message string) string {
	match := openAIResponsesInvalidTypeMessageParamPattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(match[1]))
}

func openAIResponsesCacheModelRejectionParamFromMessage(message string) string {
	match := openAIResponsesCacheModelRejectionPattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(match[1]))
}

func isExplicitOpenAIResponsesNullContentRejection(code, message string) bool {
	code = strings.TrimSpace(code)
	return (code == "invalid_type" || code == "invalid_request_error" || code == "") &&
		openAIResponsesInvalidTypeMessageParamPattern.MatchString(strings.TrimSpace(message))
}

func openAIResponsesRejectedNamespaceIndex(param string) (int, bool) {
	return openAIResponsesRejectedInputIndex(openAIResponsesRejectedNamespaceParamPattern, param)
}

func openAIResponsesRejectedStatusIndex(param string) (int, bool) {
	return openAIResponsesRejectedInputIndex(openAIResponsesRejectedStatusParamPattern, param)
}

func openAIResponsesRejectedContentIndex(param string) (int, bool) {
	return openAIResponsesRejectedInputIndex(openAIResponsesRejectedContentParamPattern, param)
}

func openAIResponsesRejectedCacheIndex(param string) (int, bool) {
	return openAIResponsesRejectedInputIndex(openAIResponsesRejectedCacheParamPattern, param)
}

func openAIResponsesRejectedIDIndex(param string) (int, bool) {
	match := openAIResponsesRejectedIDParamPattern.FindStringSubmatch(strings.TrimSpace(param))
	if len(match) != 3 {
		return 0, false
	}
	rawIndex := match[1]
	if rawIndex == "" {
		rawIndex = match[2]
	}
	index, err := strconv.Atoi(rawIndex)
	if err != nil || index < 0 {
		return 0, false
	}
	return index, true
}

func openAIResponsesInvalidValueIDIndexFromMessage(message string) (int, bool) {
	match := openAIResponsesInvalidValueIDMessagePattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return 0, false
	}
	return openAIResponsesRejectedIDIndex(match[1])
}

func openAIResponsesRejectedCallIDIndex(param string) (int, bool) {
	match := openAIResponsesRejectedCallIDParamPattern.FindStringSubmatch(strings.TrimSpace(param))
	if len(match) != 3 {
		return 0, false
	}
	rawIndex := match[1]
	if rawIndex == "" {
		rawIndex = match[2]
	}
	index, err := strconv.Atoi(rawIndex)
	if err != nil || index < 0 {
		return 0, false
	}
	return index, true
}

func openAIResponsesInvalidValueCallIDIndexFromMessage(message string) (int, bool) {
	match := openAIResponsesInvalidValueCallIDMessagePattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return 0, false
	}
	return openAIResponsesRejectedCallIDIndex(match[1])
}

func openAIResponsesRejectedRoleIndex(param string) (int, bool) {
	match := openAIResponsesRejectedRoleParamPattern.FindStringSubmatch(strings.TrimSpace(param))
	if len(match) != 3 {
		return 0, false
	}
	rawIndex := match[1]
	if rawIndex == "" {
		rawIndex = match[2]
	}
	index, err := strconv.Atoi(rawIndex)
	if err != nil || index < 0 {
		return 0, false
	}
	return index, true
}

func openAIResponsesInvalidValueRoleIndexFromMessage(message string) (int, bool) {
	match := openAIResponsesInvalidValueRoleMessagePattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return 0, false
	}
	return openAIResponsesRejectedRoleIndex(match[1])
}

func openAIResponsesRejectedInputToolsIndex(param string) (int, bool) {
	return openAIResponsesRejectedInputIndex(openAIResponsesRejectedInputToolsParamPattern, param)
}

func openAIResponsesInvalidValueInputToolsIndexFromMessage(message string) (int, bool) {
	match := openAIResponsesInvalidValueInputToolsMessagePattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return 0, false
	}
	return openAIResponsesRejectedInputToolsIndex(match[1])
}

func openAIResponsesRejectedContentElementIndices(param string) (int, int, bool) {
	match := openAIResponsesRejectedContentElementParamPattern.FindStringSubmatch(strings.TrimSpace(param))
	if len(match) != 4 {
		return 0, 0, false
	}
	itemIndex, err := strconv.Atoi(firstNonEmptyString(match[1], match[2]))
	if err != nil || itemIndex < 0 {
		return 0, 0, false
	}
	elementIndex, err := strconv.Atoi(match[3])
	if err != nil || elementIndex < 0 {
		return 0, 0, false
	}
	return itemIndex, elementIndex, true
}

func openAIResponsesInvalidValueContentElementIndicesFromMessage(message string) (int, int, bool) {
	match := openAIResponsesInvalidTypeContentElementMessagePattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return 0, 0, false
	}
	return openAIResponsesRejectedContentElementIndices(match[1])
}

func openAIResponsesRejectedInputIndex(pattern *regexp.Regexp, param string) (int, bool) {
	match := pattern.FindStringSubmatch(strings.TrimSpace(param))
	if len(match) != 2 && len(match) != 3 {
		return 0, false
	}
	rawIndex := match[1]
	if rawIndex == "" && len(match) == 3 {
		rawIndex = match[2]
	}
	index, err := strconv.Atoi(rawIndex)
	if err == nil && index >= 0 {
		return index, true
	}
	return 0, false
}

// removeOpenAIResponsesRejectedStatusAtIndex drops the status field the
// upstream rejected, and the status of every other input item sharing the
// rejected item's type.
//
// The upstream names one offending index per response, but a replayed
// conversation routinely carries dozens of items of the same type, each with a
// status its schema does not accept. Clearing one index per round trip would
// need one retry per item and exhaust the bounded retry budget long before the
// request could succeed. Items of other types keep their status: the rejection
// only proves that this type has no status field.
func removeOpenAIResponsesRejectedStatusAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", index)
	rejected := gjson.GetBytes(body, itemPath)
	if !rejected.IsObject() {
		return nil, "", false, nil
	}
	if !gjson.GetBytes(body, itemPath+".status").Exists() {
		return nil, "", false, nil
	}

	retryBody := body
	cleared := 0
	rejectedType := strings.TrimSpace(rejected.Get("type").String())
	if input := gjson.GetBytes(body, "input"); rejectedType != "" && input.IsArray() {
		// Deleting a field never shifts array indexes, so positions read from
		// the original body stay valid against the rewritten one.
		for itemIndex, item := range input.Array() {
			if !item.IsObject() || strings.TrimSpace(item.Get("type").String()) != rejectedType {
				continue
			}
			statusPath := fmt.Sprintf("input.%d.status", itemIndex)
			if !gjson.GetBytes(retryBody, statusPath).Exists() {
				continue
			}
			next, err := sjson.DeleteBytes(retryBody, statusPath)
			if err != nil {
				return nil, "", false, fmt.Errorf("delete rejected status at input[%d]: %w", itemIndex, err)
			}
			retryBody = next
			cleared++
		}
	}
	if cleared == 0 {
		// The rejected item carries no type to match on; fall back to clearing
		// just the index the upstream named.
		next, err := sjson.DeleteBytes(retryBody, itemPath+".status")
		if err != nil {
			return nil, "", false, fmt.Errorf("delete rejected status at input[%d]: %w", index, err)
		}
		retryBody = next
	}
	return retryBody, "indexed status parameter rejection", true, nil
}

func removeOpenAIResponsesRejectedCacheAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", index)
	if !gjson.GetBytes(body, itemPath).IsObject() {
		return nil, "", false, nil
	}
	cachePath := itemPath + ".prompt_cache_breakpoint"
	if !gjson.GetBytes(body, cachePath).Exists() {
		return nil, "", false, nil
	}
	retryBody, err := sjson.DeleteBytes(body, cachePath)
	if err != nil {
		return nil, "", false, fmt.Errorf("delete rejected prompt_cache_breakpoint at input[%d]: %w", index, err)
	}
	return retryBody, "indexed prompt_cache_breakpoint parameter rejection", true, nil
}

func removeOpenAIResponsesRejectedIDAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return nil, "", false, nil
	}
	items := input.Array()
	if index < 0 || index >= len(items) || !items[index].IsObject() {
		return nil, "", false, nil
	}

	item := items[index]
	itemType := strings.TrimSpace(item.Get("type").String())
	if strings.EqualFold(itemType, "item_reference") {
		rebuiltInput := make([]byte, 0, len(input.Raw))
		rebuiltInput = append(rebuiltInput, '[')
		for itemIndex, candidate := range items {
			if itemIndex == index {
				continue
			}
			if len(rebuiltInput) > 1 {
				rebuiltInput = append(rebuiltInput, ',')
			}
			rebuiltInput = append(rebuiltInput, candidate.Raw...)
		}
		rebuiltInput = append(rebuiltInput, ']')
		retryBody, err := sjson.SetRawBytes(body, "input", rebuiltInput)
		if err != nil {
			return nil, "", false, fmt.Errorf("drop rejected item_reference at input[%d]: %w", index, err)
		}
		return retryBody, "indexed item reference ID rejection", true, nil
	}

	if _, exists := item.Map()["id"]; !exists {
		if strings.EqualFold(itemType, "reasoning") {
			return removeOpenAIResponsesMissingIDReasoningItems(body, input, items)
		}
		return nil, "", false, nil
	}
	retryBody, err := sjson.DeleteBytes(body, fmt.Sprintf("input.%d.id", index))
	if err != nil {
		return nil, "", false, fmt.Errorf("delete rejected ID at input[%d]: %w", index, err)
	}
	return retryBody, "indexed input item ID rejection", true, nil
}

// A store=false compatibility transform can preserve encrypted reasoning while
// removing its server-scoped rs_* identifier. Some Responses-compatible API-key
// upstreams reject that portable form and require input[N].id. Once such an
// upstream explicitly rejects a missing reasoning ID, the item cannot be safely
// replayed: fabricating an ID could reference unrelated server state. Remove all
// missing-ID reasoning items in one bounded retry while preserving every other
// input item verbatim.
func removeOpenAIResponsesMissingIDReasoningItems(body []byte, input gjson.Result, items []gjson.Result) ([]byte, string, bool, error) {
	rebuiltInput := make([]byte, 0, len(input.Raw))
	rebuiltInput = append(rebuiltInput, '[')
	dropped := 0
	for _, candidate := range items {
		candidateType := strings.TrimSpace(candidate.Get("type").String())
		_, hasID := candidate.Map()["id"]
		if candidate.IsObject() && strings.EqualFold(candidateType, "reasoning") && !hasID {
			dropped++
			continue
		}
		if len(rebuiltInput) > 1 {
			rebuiltInput = append(rebuiltInput, ',')
		}
		rebuiltInput = append(rebuiltInput, candidate.Raw...)
	}
	rebuiltInput = append(rebuiltInput, ']')
	if dropped == 0 {
		return nil, "", false, nil
	}
	retryBody, err := sjson.SetRawBytes(body, "input", rebuiltInput)
	if err != nil {
		return nil, "", false, fmt.Errorf("drop reasoning items rejected for missing ID: %w", err)
	}
	return retryBody, "missing reasoning item ID rejection", true, nil
}

// Missing call IDs cannot be reconstructed without risking a mismatched tool
// result. After an explicit indexed rejection, drop invalid items of that same
// type and keep all valid calls, outputs, messages, and unrelated item types.
func removeOpenAIResponsesMissingCallIDItems(body []byte, index int) ([]byte, string, bool, error) {
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return nil, "", false, nil
	}
	items := input.Array()
	if index < 0 || index >= len(items) || !items[index].IsObject() {
		return nil, "", false, nil
	}
	rejectedType := strings.TrimSpace(items[index].Get("type").String())
	if rejectedType == "" || !isCodexToolCallItemType(rejectedType) || hasValidOpenAIResponsesCallID(items[index]) {
		return nil, "", false, nil
	}

	rebuiltInput := make([]byte, 0, len(input.Raw))
	rebuiltInput = append(rebuiltInput, '[')
	dropped := 0
	for _, candidate := range items {
		candidateType := strings.TrimSpace(candidate.Get("type").String())
		if candidate.IsObject() && candidateType == rejectedType && !hasValidOpenAIResponsesCallID(candidate) {
			dropped++
			continue
		}
		if len(rebuiltInput) > 1 {
			rebuiltInput = append(rebuiltInput, ',')
		}
		rebuiltInput = append(rebuiltInput, candidate.Raw...)
	}
	rebuiltInput = append(rebuiltInput, ']')
	if dropped == 0 {
		return nil, "", false, nil
	}
	retryBody, err := sjson.SetRawBytes(body, "input", rebuiltInput)
	if err != nil {
		return nil, "", false, fmt.Errorf("drop %s items rejected for missing call_id: %w", rejectedType, err)
	}
	return retryBody, "missing tool call ID rejection", true, nil
}

func hasValidOpenAIResponsesCallID(item gjson.Result) bool {
	callID := item.Get("call_id")
	return callID.Exists() && callID.Type == gjson.String && strings.TrimSpace(callID.String()) != ""
}

func removeOpenAIResponsesRejectedInputToolsAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", index)
	item := gjson.GetBytes(body, itemPath)
	if !item.IsObject() || !item.Get("tools").Exists() {
		return nil, "", false, nil
	}
	retryBody, err := sjson.DeleteBytes(body, itemPath+".tools")
	if err != nil {
		return nil, "", false, fmt.Errorf("delete rejected input item tools at input[%d]: %w", index, err)
	}
	return retryBody, "indexed input item tools rejection", true, nil
}

func removeOpenAIResponsesRejectedNullContentElementAtIndex(body []byte, itemIndex, elementIndex int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", itemIndex)
	item := gjson.GetBytes(body, itemPath)
	content := item.Get("content")
	if !item.IsObject() || !content.IsArray() {
		return nil, "", false, nil
	}
	elements := content.Array()
	if elementIndex < 0 || elementIndex >= len(elements) || elements[elementIndex].Type != gjson.Null {
		return nil, "", false, nil
	}
	rebuilt := make([]byte, 0, len(content.Raw))
	rebuilt = append(rebuilt, '[')
	dropped := 0
	for _, element := range elements {
		if element.Type == gjson.Null {
			dropped++
			continue
		}
		if len(rebuilt) > 1 {
			rebuilt = append(rebuilt, ',')
		}
		rebuilt = append(rebuilt, element.Raw...)
	}
	rebuilt = append(rebuilt, ']')
	retryBody, err := sjson.SetRawBytes(body, itemPath+".content", rebuilt)
	if err != nil {
		return nil, "", false, fmt.Errorf("drop rejected null content elements at input[%d]: %w", itemIndex, err)
	}
	return retryBody, "indexed null content element rejection", dropped > 0, nil
}

// A message item cannot be replayed without a role and the role cannot be
// reconstructed safely from its content. After the upstream explicitly names
// one invalid role, drop only role-less message items (including the named
// item); preserve valid messages and non-message items. For malformed
// non-message items, remove the stray role field instead of dropping the tool
// call or output itself.
func removeOpenAIResponsesMissingRoleItems(body []byte, index int) ([]byte, string, bool, error) {
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return nil, "", false, nil
	}
	items := input.Array()
	if index < 0 || index >= len(items) || !items[index].IsObject() {
		return nil, "", false, nil
	}
	rejected := items[index]
	rejectedType := strings.TrimSpace(rejected.Get("type").String())
	role := rejected.Get("role")
	roleValid := role.Exists() && role.Type == gjson.String && strings.TrimSpace(role.String()) != ""
	if roleValid {
		return nil, "", false, nil
	}

	if strings.EqualFold(rejectedType, "message") || rejectedType == "" {
		rebuiltInput := make([]byte, 0, len(input.Raw))
		rebuiltInput = append(rebuiltInput, '[')
		dropped := 0
		for _, candidate := range items {
			candidateType := strings.TrimSpace(candidate.Get("type").String())
			candidateRole := candidate.Get("role")
			candidateRoleValid := candidateRole.Exists() && candidateRole.Type == gjson.String && strings.TrimSpace(candidateRole.String()) != ""
			missingRole := candidate.IsObject() && !candidateRoleValid &&
				(strings.EqualFold(candidateType, rejectedType) || (rejectedType == "" && candidateType == ""))
			if missingRole {
				dropped++
				continue
			}
			if len(rebuiltInput) > 1 {
				rebuiltInput = append(rebuiltInput, ',')
			}
			rebuiltInput = append(rebuiltInput, candidate.Raw...)
		}
		rebuiltInput = append(rebuiltInput, ']')
		if dropped == 0 {
			return nil, "", false, nil
		}
		retryBody, err := sjson.SetRawBytes(body, "input", rebuiltInput)
		if err != nil {
			return nil, "", false, fmt.Errorf("drop %s items rejected for missing role: %w", rejectedType, err)
		}
		return retryBody, "missing message item role rejection", true, nil
	}

	// A role on a non-message item is not part of its Responses shape. If the
	// field is null/empty, remove that field and preserve the rest of the item.
	// When the field is absent, deleting it would be a no-op and the retry guard
	// would correctly reject the unchanged body. Drop only the indexed malformed
	// item in that case; its role cannot be reconstructed safely.
	if !role.Exists() {
		retryBody, err := removeOpenAIResponsesInputItemAtIndex(body, index)
		if err != nil {
			return nil, "", false, fmt.Errorf("drop role-less input item at input[%d]: %w", index, err)
		}
		return retryBody, "drop role-less input item rejection", true, nil
	}
	rolePath := fmt.Sprintf("input.%d.role", index)
	retryBody, err := sjson.DeleteBytes(body, rolePath)
	if err != nil {
		return nil, "", false, fmt.Errorf("delete rejected role at input[%d]: %w", index, err)
	}
	return retryBody, "stray non-message role rejection", true, nil
}

func removeOpenAIResponsesInputItemAtIndex(body []byte, index int) ([]byte, error) {
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() || index < 0 || index >= len(input.Array()) {
		return nil, fmt.Errorf("input index %d is out of range", index)
	}
	items := input.Array()
	rebuiltInput := make([]byte, 0, len(input.Raw))
	rebuiltInput = append(rebuiltInput, '[')
	for itemIndex, item := range items {
		if itemIndex == index {
			continue
		}
		if len(rebuiltInput) > 1 {
			rebuiltInput = append(rebuiltInput, ',')
		}
		rebuiltInput = append(rebuiltInput, item.Raw...)
	}
	rebuiltInput = append(rebuiltInput, ']')
	return sjson.SetRawBytes(body, "input", rebuiltInput)
}

func normalizeOpenAIResponsesRejectedNullContentAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", index)
	item := gjson.GetBytes(body, itemPath)
	content := gjson.GetBytes(body, itemPath+".content")
	if !item.IsObject() || !content.Exists() || content.Type != gjson.Null {
		return nil, "", false, nil
	}

	itemType := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
	role := strings.TrimSpace(item.Get("role").String())
	contentPath := itemPath + ".content"
	switch {
	case itemType == "reasoning":
		retryBody, err := sjson.DeleteBytes(body, contentPath)
		if err != nil {
			return nil, "", false, fmt.Errorf("delete rejected null content at input[%d]: %w", index, err)
		}
		return retryBody, "indexed reasoning null content rejection", true, nil
	case itemType == "message" || role != "":
		retryBody, err := sjson.SetBytes(body, contentPath, "")
		if err != nil {
			return nil, "", false, fmt.Errorf("normalize rejected null content at input[%d]: %w", index, err)
		}
		return retryBody, "indexed message null content rejection", true, nil
	default:
		return nil, "", false, nil
	}
}

func removeOpenAIResponsesRejectedReasoningContentAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", index)
	item := gjson.GetBytes(body, itemPath)
	content := item.Get("content")
	if !item.IsObject() || strings.TrimSpace(item.Get("type").String()) != "reasoning" || !content.IsArray() || len(content.Array()) == 0 {
		return nil, "", false, nil
	}
	retryBody, err := sjson.DeleteBytes(body, itemPath+".content")
	if err != nil {
		return nil, "", false, fmt.Errorf("delete rejected reasoning content at input[%d]: %w", index, err)
	}
	return retryBody, "indexed reasoning content maximum-length rejection", true, nil
}

func removeOpenAIResponsesRejectedNamespaceAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", index)
	itemType := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, itemPath+".type").String()))
	switch itemType {
	case "function_call", "tool_call", "custom_tool_call", "mcp_tool_call":
	default:
		return nil, "", false, nil
	}

	namespacePath := itemPath + ".namespace"
	if !gjson.GetBytes(body, namespacePath).Exists() {
		return nil, "", false, nil
	}
	retryBody, err := sjson.DeleteBytes(body, namespacePath)
	if err != nil {
		return nil, "", false, fmt.Errorf("delete rejected namespace at input[%d]: %w", index, err)
	}
	return retryBody, "indexed namespace parameter rejection", true, nil
}
