const templates = new Map<string, HTMLTemplateElement>()

export function cachedMarkup(html: string): Node {
  let template = templates.get(html)
  if (!template) {
    template = document.createElement('template')
    template.innerHTML = html
    templates.set(html, template)
  }
  return template.content.cloneNode(true)
}
