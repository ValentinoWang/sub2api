/** Resolve tutorial-owned resources without baking a deployment host into content. */
export function formatTutorialPrompt(prompt, pageUrl) {
  const page = new URL(pageUrl)
  if (!['http:', 'https:', 'file:'].includes(page.protocol)) {
    throw new TypeError('Unsupported tutorial page URL')
  }
  // Share the page location, never authentication, referral queries or fragments.
  page.username = ''
  page.password = ''
  page.search = ''
  page.hash = ''
  const links = /(\[[^\]\n]+\]\()((?:\/(?!\/)|\.\.?\/)[^\s)]+)(\))/g
  const resolved = prompt.replace(links, (_match, label, path, end) =>
    `${label}${new URL(path, page).href}${end}`)
  return `${resolved}\n\n教程来源：${page.href}`
}
