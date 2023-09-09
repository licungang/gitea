const {csrfToken} = window.config;

function request(url, {headers, json, ...other} = {}) {
  return window.fetch(url, {
    headers: {
      'x-csrf-token': csrfToken,
      ...(json && {'content-type': 'application/json'}),
      ...headers,
    },
    ...(json && {body: JSON.stringify(json)}),
    ...other,
  });
}

export const GET = (url, opts) => request(url, {method: 'GET', ...opts});
export const POST = (url, opts) => request(url, {method: 'POST', ...opts});
export const PATCH = (url, opts) => request(url, {method: 'PATCH', ...opts});
export const PUT = (url, opts) => request(url, {method: 'PUT', ...opts});
export const DELETE = (url, opts) => request(url, {method: 'DELETE', ...opts});

// network errors are currently only detectable by error message
// based on https://github.com/sindresorhus/p-retry/blob/main/index.js
const networkErrorMsgs = new Set([
  'Failed to fetch', // Chrome
  'NetworkError when attempting to fetch resource.', // Firefox
  'The Internet connection appears to be offline.', // Safari
]);

export function isNetworkError(msg) {
  return networkErrorMsgs.has(msg);
}
