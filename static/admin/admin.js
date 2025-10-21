async function api(url, opts={}) {
  const res = await fetch(url, Object.assign({headers:{'Content-Type':'application/json'}}, opts));
  const data = await res.json().catch(()=>({}));
  if (!res.ok || data.status !== 'success') throw new Error(data.message || 'Ошибка запроса');
  return data.data;
}

async function loadCustomers() {
  const list = await api('/admin/customers');
  const tbody = document.querySelector('#customersTable tbody');
  tbody.innerHTML='';
  (list||[]).forEach(c => {
    const tr = document.createElement('tr');
    tr.innerHTML = `
      <td contenteditable="false"><code style="font-size:12px">${c.id}</code> <button class="btn secondary" data-copy="${c.id}">Копировать</button></td>
      <td contenteditable data-field="email" data-id="${c.id}">${c.email}</td>
      <td contenteditable data-field="name" data-id="${c.id}">${c.name||''}</td>
      <td contenteditable data-field="company" data-id="${c.id}">${c.company||''}</td>
      <td contenteditable data-field="status" data-id="${c.id}">${c.status}</td>
      <td>${c.createdAt}</td>
      <td style="text-align:right;">
        <button class="btn secondary" data-save="${c.id}">Сохранить</button>
        <button class="btn" title="Перегенерировать пароль" data-repass="${c.id}">🔁 Пароль</button>
        <button class="btn danger" data-del="${c.id}">Удалить</button>
      </td>`;
    tbody.appendChild(tr);
  });
}

async function createCustomer() {
  const email = prompt('Email клиента:');
  if (!email) return;
  const password = prompt('Пароль (будет захеширован):');
  if (!password) return;
  const name = prompt('Имя (опционально):')||'';
  const company = prompt('Компания (опционально):')||'';
  await api('/admin/customers/create', {method:'POST', body: JSON.stringify({email,password,name,company})});
  await loadCustomers();
}

async function updateCustomer(id) {
  const email = prompt('Новый email (пусто — без изменений):');
  const name = prompt('Имя (пусто — без изменений):');
  const company = prompt('Компания (пусто — без изменений):');
  const status = prompt('Статус (active/blocked, пусто — без изменений):');
  const password = prompt('Пароль (пусто — без изменений):');
  const payload = {id};
  if (email) payload.email = email;
  if (name) payload.name = name;
  if (company) payload.company = company;
  if (status) payload.status = status;
  if (password) payload.password = password;
  await api('/admin/customers/update', {method:'POST', body: JSON.stringify(payload)});
  await loadCustomers();
}

async function deleteCustomer(id) {
  if (!confirm('Удалить клиента (soft delete)?')) return;
  await api('/admin/customers/delete', {method:'POST', body: JSON.stringify({id})});
  await loadCustomers();
}

async function createSubscription() {
  const customer_id = document.getElementById('subCustomerId').value.trim();
  const quota = parseInt(document.getElementById('subQuota').value||'0',10);
  const start = document.getElementById('subStart').value;
  const end = document.getElementById('subEnd').value;
  if (!customer_id || !quota || !start || !end) { alert('Заполните все поля'); return; }
  // фронт-валидация дат: end>=start и end не в прошлом
  const s = new Date(start + 'T00:00:00Z');
  const e = new Date(end + 'T00:00:00Z');
  const today = new Date(); today.setUTCHours(0,0,0,0);
  if (e < s) { alert('Дата окончания должна быть не раньше даты начала'); return; }
  if (e < today) { alert('Дата окончания не может быть в прошлом'); return; }
  await api('/admin/subscriptions/create', {method:'POST', body: JSON.stringify({customer_id, quota_total:quota, period_start:start, period_end:end})});
  alert('Подписка создана');
}

document.addEventListener('DOMContentLoaded', () => {
// Toggle pretty create form
const createForm = document.getElementById('createCustomerForm');
if (createForm){
  document.getElementById('createCustomerBtn').addEventListener('click', ()=>{ createForm.style.display='block'; });
  document.getElementById('cancelCreate').addEventListener('click', ()=>{ createForm.style.display='none'; });
  createForm.addEventListener('submit', async (e)=>{
    e.preventDefault();
    const email = document.getElementById('newEmail').value.trim();
    const name = document.getElementById('newName').value.trim();
    const company = document.getElementById('newCompany').value.trim();
    if(!email){ showToast('Email обязателен'); return; }
    try{
      const data = await api('/admin/customers/create', {method:'POST', body: JSON.stringify({email,name,company})});
      createForm.reset(); createForm.style.display='none';
      await loadCustomers();
      const pwd = data && data.generatedPassword ? data.generatedPassword : '(нет)';
      showModal(`<div>Email: <b>${email}</b></div><div>Пароль: <b>${pwd}</b></div><div class="muted">Сохраните эти данные — повторно показать пароль будет невозможно.</div>`);
    }catch(e){ showToast(e.message||'Ошибка'); }
  });
} else {
  document.getElementById('createCustomerBtn').addEventListener('click', createCustomer);
}
  document.getElementById('createSubBtn').addEventListener('click', createSubscription);
  document.getElementById('logoutBtn').addEventListener('click', async () => { await api('/admin/logout', {method:'POST'}); location.href = '/admin/login.html'; });
document.querySelector('#customersTable').addEventListener('click', async (e) => {
    const t = e.target;
  if (t.dataset.save) {
    const id = t.dataset.save;
    const row = t.closest('tr');
    const get = (field) => row.querySelector(`[data-field="${field}"][data-id="${id}"]`).innerText.trim();
    const payload = { id };
    const email = get('email'); if (email) payload.email = email;
    const name = get('name'); if (name) payload.name = name;
    const company = get('company'); if (company) payload.company = company;
    const status = get('status'); if (status) payload.status = status;
    await api('/admin/customers/update', { method:'POST', body: JSON.stringify(payload) });
    alert('Сохранено');
  }
    if (t.dataset.del) deleteCustomer(t.dataset.del);
  if (t.dataset.copy) {
    copyToClipboard(t.dataset.copy).then((ok)=>{
      showToast(ok ? 'ID скопирован' : 'Не удалось скопировать ID');
    });
  }
  if (t.dataset.repass) regeneratePassword(t.dataset.repass);
  });
  loadCustomers().catch(err => alert(err.message));
});

// UI helpers
function showToast(msg){
  const t = document.getElementById('toast');
  t.textContent = msg; t.style.display='block';
  setTimeout(()=>{ t.style.display='none'; }, 2500);
}
async function copyToClipboard(text){
  try{
    if (navigator && navigator.clipboard && navigator.clipboard.writeText){
      await navigator.clipboard.writeText(text);
      return true;
    }
  }catch(_){/* fallback below */}
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.style.position = 'fixed';
  ta.style.left = '-9999px';
  document.body.appendChild(ta);
  ta.focus();
  ta.select();
  let ok = false;
  try{ ok = document.execCommand('copy'); }catch(_){ ok = false; }
  document.body.removeChild(ta);
  return ok;
}
function showModal(html){
  const m = document.getElementById('modal');
  const c = document.getElementById('modalContent');
  c.innerHTML = html;
  m.style.display='flex';
  document.getElementById('modalCopy').onclick = async () => {
    const ok = await copyToClipboard(c.innerText);
    showToast(ok ? 'Скопировано' : 'Не удалось скопировать');
  };
  document.getElementById('modalClose').onclick = () => { m.style.display='none'; };
}

// Override createCustomer to auto-generate password and show modal
async function createCustomer() {
  const email = prompt('Email клиента:');
  if (!email) return;
  const name = prompt('Имя (опционально):')||'';
  const company = prompt('Компания (опционально):')||'';
  try{
    const data = await api('/admin/customers/create', {method:'POST', body: JSON.stringify({email,name,company})});
    await loadCustomers();
    const pwd = data && data.generatedPassword ? data.generatedPassword : '(нет)';
    showModal(`<div>Email: <b>${email}</b></div><div>Пароль: <b>${pwd}</b></div><div class="muted">Сохраните эти данные — повторно показать пароль будет невозможно.</div>`);
  }catch(e){
    showToast(e.message||'Ошибка');
  }
}

// Regenerate password for customer
async function regeneratePassword(customerId){
  try{
    const data = await api('/admin/customers/regenerate_password', {method:'POST', body: JSON.stringify({id: customerId})});
    const pwd = data && data.generatedPassword ? data.generatedPassword : '(нет)';
    showModal(`<div>Новый пароль: <b>${pwd}</b></div><div class="muted">Показывается один раз — сообщите пользователю и сохраните.</div>`);
  }catch(e){ showToast(e.message||'Ошибка'); }
}


