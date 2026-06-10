// Apply persisted theme before paint to prevent flash
const t = localStorage.getItem('dispatch-theme');
if (t) document.documentElement.setAttribute('data-theme', t);
