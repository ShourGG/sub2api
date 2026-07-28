/**
 * Stub: modelDisplay — model-name formatting helpers.
 */
export function displayModelLabel(modelId: string, fallbackName?: string): string {
  return fallbackName || modelId
}
export function formatModelDisplayName(modelId: string): string {
  return modelId
}
export function getModelShortName(modelId: string): string {
  const parts = modelId.split('/')
  return parts[parts.length - 1] ?? modelId
}
export function isImageModel(modelId: string): boolean {
  const id = modelId.toLowerCase()
  return id.includes('image') || id.includes('dall-e') || id.includes('gpt-image')
}
