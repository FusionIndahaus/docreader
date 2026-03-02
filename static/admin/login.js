document.addEventListener('DOMContentLoaded', () => {
  const form = document.getElementById('loginForm');
  const err = document.getElementById('err');
  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    err.textContent = '';
    const email = document.getElementById('email').value.trim();
    const password = document.getElementById('password').value;
    const body = new URLSearchParams();
    body.append('email', email);
    body.append('password', password);
    const res = await fetch('/admin/login', { method: 'POST', body });
    const json = await res.json().catch(() => ({}));
    if (!res.ok || json.status !== 'success') {
      err.textContent = json.message || 'Ошибка входа';
      return;
    }
    location.href = '/admin/';
  });
});


