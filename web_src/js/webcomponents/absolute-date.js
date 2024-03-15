import {Temporal} from 'temporal-polyfill';

window.customElements.define('absolute-date', class extends HTMLElement {
  static observedAttributes = ['date', 'year', 'month', 'weekday', 'day'];

  update = () => {
    const year = this.getAttribute('year') ?? '';
    const month = this.getAttribute('month') ?? '';
    const weekday = this.getAttribute('weekday') ?? '';
    const day = this.getAttribute('day') ?? '';
    const lang = this.closest('[lang]')?.getAttribute('lang') ||
      this.ownerDocument.documentElement.getAttribute('lang') ||
      '';

    // only use the first 10 characters, e.g. the `yyyy-mm-dd` part
    const plainDate = Temporal.PlainDate.from(this.getAttribute('date').substring(0, 10));
    if (!this.shadowRoot) this.attachShadow({mode: 'open'});
    this.shadowRoot.textContent = plainDate.toLocaleString(lang ?? [], {
      ...(year && {year}),
      ...(month && {month}),
      ...(weekday && {weekday}),
      ...(day && {day}),
    });
  };

  attributeChangedCallback(_name, oldValue, newValue) {
    if (!this.initialized || oldValue === newValue) return;
    this.update();
  }

  connectedCallback() {
    this.initialized = false;
    this.update();
    this.initialized = true;
  }
});
