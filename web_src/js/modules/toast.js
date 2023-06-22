import {htmlEscape} from 'escape-goat';
import {svg} from '../svg.js';

const levels = {
  info: {
    icon: 'octicon-check',
    background: 'var(--color-green)',
    dur: 2500,
  },
  error: {
    icon: 'gitea-exclamation',
    background: 'var(--color-red)',
    dur: -1, // needs to be clicked away
  },
};

// See https://github.com/apvarun/toastify-js#api for options
async function showToast(message, level, {gravity, position, duration, ...other} = {}) {
  if (!message) return;

  const {default: Toastify} = await import(/* webpackChunkName: 'toastify' */'toastify-js');
  const {icon, background, dur} = levels[level ?? 'info'];

  const toast = Toastify({
    text: `
      <div class='toast-icon'>${svg(icon)}</div>
      <div class='toast-body'>
        <div class='toast-message'>${htmlEscape(message)}</div>
      </div>
      <button class='toast-close'>${svg('octicon-x')}</button>
    `,
    escapeMarkup: false,
    gravity: gravity ?? 'top',
    position: position ?? 'center',
    duration: duration ?? dur,
    style: {background},
    ...other,
  });

  toast.showToast();

  toast.toastElement.querySelector('.toast-close').addEventListener('click', () => {
    toast.removeElement(toast.toastElement);
  });
}

export async function showInfo(message, opts) {
  return await showToast(message, 'info', opts);
}

export async function showError(message, opts) {
  return await showToast(message, 'error', opts);
}

// export for devtest page in development
if (process.env.NODE_ENV === 'development') {
  window.showInfo = showInfo;
  window.showError = showError;
}
