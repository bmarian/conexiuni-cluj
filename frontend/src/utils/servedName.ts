// Set by backend/pages on the page it served.
const served = {path: location.pathname, name: document.getElementById('app')?.dataset.name ?? ''}

export const servedName = (path: string) => (path === served.path ? served.name : '')
