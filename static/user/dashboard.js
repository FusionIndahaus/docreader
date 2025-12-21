// Dashboard functionality
class UserDashboard {
    constructor() {
        this.init();
    }

    init() {
        this.initTheme(); // Инициализируем тему ПЕРЕД привязкой событий
        this.bindEvents();
        this.loadUserData();
        this.loadSettings();
        this.loadSubscriptionInfo();
        this.loadUploadHistory();
        this.loadUsageStats();
    }

    bindEvents() {
        document.getElementById('logoutBtn').addEventListener('click', () => {
            this.logout();
        });

        document.getElementById('updateProfileBtn').addEventListener('click', () => {
            this.updateProfile();
        });

        document.getElementById('changePasswordBtn').addEventListener('click', () => {
            this.changePassword();
        });

        document.getElementById('updateSettingsBtn').addEventListener('click', () => {
            this.updateSettings();
        });

        const amoConnectBtn = document.getElementById('amoConnectBtn');
        if (amoConnectBtn) {
            amoConnectBtn.addEventListener('click', () => {
                // перед подключением проверим, что BYOA креды заданы
                this.ensureAmoCredentialsThenConnect();
            });
        }
        const amoSaveBtn = document.getElementById('amoSaveSettingsBtn');
        if (amoSaveBtn) {
            amoSaveBtn.addEventListener('click', () => {
                this.saveAmoSettings();
            });
        }
        const bitrixSaveBtn = document.getElementById('bitrixSaveSettingsBtn');
        if (bitrixSaveBtn) {
            bitrixSaveBtn.addEventListener('click', () => {
                this.saveBitrixSettings();
            });
        }
        const bitrixLoadFieldsBtn = document.getElementById('bitrixLoadFieldsBtn');
        if (bitrixLoadFieldsBtn) {
            bitrixLoadFieldsBtn.addEventListener('click', () => {
                this.loadBitrixFieldsAndMapping();
            });
        }
        const bitrixSaveMappingBtn = document.getElementById('bitrixSaveMappingBtn');
        if (bitrixSaveMappingBtn) {
            bitrixSaveMappingBtn.addEventListener('click', () => {
                this.saveBitrixMapping();
            });
        }
        const amoLoadFieldsBtn = document.getElementById('amoLoadFieldsBtn');
        if (amoLoadFieldsBtn) {
            amoLoadFieldsBtn.addEventListener('click', () => {
                this.loadAmoFieldsAndMapping();
            });
        }
        const amoSaveMappingBtn = document.getElementById('amoSaveMappingBtn');
        if (amoSaveMappingBtn) {
            amoSaveMappingBtn.addEventListener('click', () => {
                this.saveAmoMapping();
            });
        }

        // Theme switcher
        const themeSwitch = document.getElementById('themeSwitch');
        if (themeSwitch) {
            themeSwitch.addEventListener('change', () => {
                this.toggleTheme();
            });
        }
    }

    async loadUserData() {
        try {
            const response = await fetch('/user/profile', {
                method: 'GET',
                credentials: 'include'
            });

            if (response.ok) {
                const data = await response.json();
                if (data.status === 'success') {
                    this.displayUserData(data.data);
                }
            }
        } catch (error) {
            console.error('Ошибка загрузки данных пользователя:', error);
        }
    }

    displayUserData(userData) {
        const nameEl = document.getElementById('userName');
        if (nameEl) {
            nameEl.textContent = userData.name || userData.email;
        }
        document.getElementById('userFullName').value = userData.name || '';
        document.getElementById('userCompany').value = userData.company || '';
    }

    async loadSubscriptionInfo() {
        try {
            const response = await fetch('/user/subscription', {
                method: 'GET',
                credentials: 'include'
            });

            if (response.ok) {
                const data = await response.json();
                if (data.status === 'success') {
                    this.displaySubscriptionInfo(data.data);
                }
            } else if (response.status === 404) {
                this.displaySubscriptionInfo({
                    status: 'нет подписки',
                    period_start: null,
                    period_end: null,
                    quota_total: 0,
                    usage_count: 0
                });
            }
        } catch (error) {
            console.error('Ошибка загрузки информации о подписке:', error);
        }
    }

    displaySubscriptionInfo(subscriptionData) {
        document.getElementById('subscriptionStatus').textContent = subscriptionData.status || 'Неизвестно';
        document.getElementById('subscriptionPeriod').textContent =
            `с ${this.formatDate(subscriptionData.period_start)} до ${this.formatDate(subscriptionData.period_end)}`;
        document.getElementById('subscriptionQuota').textContent = subscriptionData.quota_total || '0';
        document.getElementById('usageCount').textContent = subscriptionData.usage_count || '0';
    }

    formatDate(dateString) {
        if (!dateString) {
            return '—';
        }

        const date = new Date(dateString);

        if (Number.isNaN(date.getTime())) {
            return dateString;
        }

        const day = String(date.getDate()).padStart(2, '0');
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const year = date.getFullYear();

        return `${day}.${month}.${year}`;
    }

    async loadUploadHistory() {
        try {
            const response = await fetch('/user/history', {
                method: 'GET',
                credentials: 'include'
            });

            if (response.ok) {
                const data = await response.json();
                if (data.status === 'success') {
                    this.displayUploadHistory(data.data);
                }
            }
        } catch (error) {
            console.error('Ошибка загрузки истории:', error);
        }
    }

    displayUploadHistory(historyData) {
        const historyList = document.getElementById('uploadHistoryList');
        
        if (historyData.length === 0) {
            historyList.innerHTML = '<div class="no-data">История загрузок пуста</div>';
            return;
        }

        historyList.innerHTML = historyData.map(item => `
            <div class="history-item">
                <div class="history-info">
                    <div class="history-date">${new Date(item.timestamp).toLocaleString('ru-RU')}</div>
                    <div class="history-status status-${item.status}">${this.getStatusText(item.status)}</div>
                </div>
                <div class="history-details">
                    <div class="history-text">${item.text.substring(0, 100)}${item.text.length > 100 ? '...' : ''}</div>
                    ${item.download ? `<a href="${item.download}" class="download-link">Скачать</a>` : ''}
                </div>
            </div>
        `).join('');
    }

    getStatusText(status) {
        const statusMap = {
            'completed': 'Завершено',
            'processing': 'Обрабатывается',
            'error': 'Ошибка',
            'pending': 'Ожидает'
        };
        return statusMap[status] || status;
    }

    async loadUsageStats() {
        try {
            const response = await fetch('/user/usage-stats', {
                method: 'GET',
                credentials: 'include'
            });

            if (response.ok) {
                const data = await response.json();
                if (data.status === 'success') {
                    this.displayUsageStats(data.data);
                }
            }
        } catch (error) {
            console.error('Ошибка загрузки статистики:', error);
        }
    }

    displayUsageStats(statsData) {
        document.getElementById('totalProcessed').textContent = statsData.total_processed || '0';
        document.getElementById('monthlyProcessed').textContent = statsData.monthly_processed || '0';
        document.getElementById('remainingQuota').textContent = statsData.remaining_quota || '0';
    }

    async updateProfile() {
        const name = document.getElementById('userFullName').value.trim();
        const company = document.getElementById('userCompany').value.trim();

        try {
            const response = await fetch('/user/profile', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify({
                    name: name,
                    company: company
                })
            });

            if (response.ok) {
                const data = await response.json();
                if (data.status === 'success') {
                    alert('Профиль успешно обновлен');
                    this.loadUserData();
                } else {
                    alert('Ошибка обновления профиля: ' + (data.message || 'Неизвестная ошибка'));
                }
            } else {
                alert('Ошибка обновления профиля');
            }
        } catch (error) {
            console.error('Ошибка обновления профиля:', error);
            alert('Ошибка обновления профиля');
        }
    }

    async loadSettings() {
        try {
            const response = await fetch('/user/settings', {
                method: 'GET',
                credentials: 'include'
            });

            if (response.ok) {
                const data = await response.json();
                if (data.status === 'success') {
                    this.displaySettings(data.data);
                    this.loadAmoStatus(); // после загрузки настроек
                    this.loadAmoSettings(); // загрузим BYOA поля
                    this.loadBitrixSettings(); // загрузим Bitrix поля
                    // Предзагрузка маппингов (молча)
                    this.prefetchMappings();
                }
            }
        } catch (error) {
            console.error('Ошибка загрузки настроек:', error);
        }
    }

    async prefetchMappings() {
        // Пытаемся получить и отрисовать, если уже есть сохранённые маппинги
        try {
            const [bFieldsResp, bMapResp] = await Promise.all([
                fetch('/user/integrations/bitrix/fields', { credentials: 'include' }),
                fetch('/user/integrations/bitrix/mapping', { credentials: 'include' })
            ]);
            if (bFieldsResp.ok && bMapResp.ok) {
                const fields = await bFieldsResp.json();
                const mapping = await bMapResp.json();
                this.renderBitrixMapping(fields.data || [], mapping.data || {});
            }
        } catch (_) {}
        try {
            const [aFieldsResp, aMapResp] = await Promise.all([
                fetch('/user/integrations/amocrm/fields', { credentials: 'include' }),
                fetch('/user/integrations/amocrm/mapping', { credentials: 'include' })
            ]);
            if (aFieldsResp.ok && aMapResp.ok) {
                const fields = await aFieldsResp.json();
                const mapping = await aMapResp.json();
                this.renderAmoMapping(fields.data || [], mapping.data || {});
            }
        } catch (_) {}
    }

    // ===== Bitrix mapping UI =====
    async loadBitrixFieldsAndMapping() {
        try {
            const [fieldsResp, mapResp] = await Promise.all([
                fetch('/user/integrations/bitrix/fields', { credentials: 'include' }),
                fetch('/user/integrations/bitrix/mapping', { credentials: 'include' })
            ]);
            if (!fieldsResp.ok) {
                alert('Не удалось загрузить поля Bitrix');
                return;
            }
            const fieldsData = await fieldsResp.json();
            const mappingData = mapResp.ok ? await mapResp.json() : { data: {} };
            this.renderBitrixMapping(fieldsData.data || [], mappingData.data || {});
        } catch (e) {
            alert('Ошибка загрузки полей Bitrix');
        }
    }

    renderBitrixMapping(fields, conf) {
        const container = document.getElementById('bitrixFieldsMapping');
        if (!container) return;
        // Подготовим опции
        const selects = [1,2,3,4,5].map(i => document.getElementById(`bitrixMap${i}`));
        const mkOption = (value, label) => {
            const opt = document.createElement('option');
            opt.value = value;
            opt.textContent = label;
            return opt;
        };
        const fillSelect = (selectEl) => {
            if (!selectEl) return;
            selectEl.innerHTML = '';
            selectEl.appendChild(mkOption('', '— не заполнять —'));
            // стандартные часто используемые
            selectEl.appendChild(mkOption('OPPORTUNITY', 'OPPORTUNITY (сумма)'));
            selectEl.appendChild(mkOption('TITLE', 'TITLE (заголовок)'));
            selectEl.appendChild(mkOption('COMMENTS', 'COMMENTS (комментарии)'));
            // поля из Bitrix
            (fields || []).forEach(f => {
                selectEl.appendChild(mkOption(f.code, `${f.title || f.code} [${f.code}]`));
            });
        };
        selects.forEach(fillSelect);
        // Проставим сохранённые значения
        const map = (conf && conf.lead_field_map_by_index) || {};
        [1,2,3,4,5].forEach(i => {
            const key = String(i);
            const selectEl = document.getElementById(`bitrixMap${i}`);
            if (selectEl && map[key]) {
                selectEl.value = map[key];
            }
        });
        container.style.display = 'block';
    }

    async saveBitrixMapping() {
        const map = {};
        [1,2,3,4,5].forEach(i => {
            const v = (document.getElementById(`bitrixMap${i}`) || {}).value || '';
            if (v) map[String(i)] = v;
        });
        try {
            const r = await fetch('/user/integrations/bitrix/mapping', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify({ lead_field_map_by_index: map })
            });
            const data = await r.json();
            if (r.ok && data.status === 'success') {
                alert('Маппинг Bitrix сохранён');
            } else {
                alert('Ошибка сохранения маппинга Bitrix');
            }
        } catch (e) {
            alert('Ошибка сети при сохранении маппинга Bitrix');
        }
    }

    // ===== amoCRM mapping UI =====
    async loadAmoFieldsAndMapping() {
        try {
            const [fieldsResp, mapResp] = await Promise.all([
                fetch('/user/integrations/amocrm/fields', { credentials: 'include' }),
                fetch('/user/integrations/amocrm/mapping', { credentials: 'include' })
            ]);
            if (!fieldsResp.ok) {
                alert('Не удалось загрузить поля amoCRM');
                return;
            }
            const fieldsData = await fieldsResp.json();
            const mappingData = mapResp.ok ? await mapResp.json() : { data: {} };
            this.renderAmoMapping(fieldsData.data || [], mappingData.data || {});
        } catch (e) {
            alert('Ошибка загрузки полей amoCRM');
        }
    }

    renderAmoMapping(fields, conf) {
        const container = document.getElementById('amoFieldsMapping');
        if (!container) return;
        const selects = [1,2,3,4,5].map(i => document.getElementById(`amoMap${i}`));
        const mkOption = (value, label) => {
            const opt = document.createElement('option');
            opt.value = value;
            opt.textContent = label;
            return opt;
        };
        const fillSelect = (selectEl) => {
            if (!selectEl) return;
            selectEl.innerHTML = '';
            selectEl.appendChild(mkOption('', '— не заполнять —'));
            // стандартные
            selectEl.appendChild(mkOption('name', 'name (Название сделки)'));
            selectEl.appendChild(mkOption('price', 'price (Бюджет)'));
            (fields || []).forEach(f => {
                if (f.scope === 'custom') {
                    selectEl.appendChild(mkOption(`cf:${f.id}`, `${f.name} [${f.id}]`));
                }
            });
        };
        selects.forEach(fillSelect);
        const map = (conf && conf.lead_field_map_by_index) || {};
        [1,2,3,4,5].forEach(i => {
            const key = String(i);
            const selectEl = document.getElementById(`amoMap${i}`);
            if (selectEl && map[key]) {
                selectEl.value = map[key];
            }
        });
        container.style.display = 'block';
    }

    async saveAmoMapping() {
        const map = {};
        [1,2,3,4,5].forEach(i => {
            const v = (document.getElementById(`amoMap${i}`) || {}).value || '';
            if (v) map[String(i)] = v;
        });
        try {
            const r = await fetch('/user/integrations/amocrm/mapping', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify({ lead_field_map_by_index: map })
            });
            const data = await r.json();
            if (r.ok && data.status === 'success') {
                alert('Маппинг amoCRM сохранён');
            } else {
                alert('Ошибка сохранения маппинга amoCRM');
            }
        } catch (e) {
            alert('Ошибка сети при сохранении маппинга amoCRM');
        }
    }
    displaySettings(settings) {
        // Получаем список разрешенных форматов
        const outputFormats = settings.output_formats || ['csv', 'xlsx', 'json'];
        
        // Устанавливаем чекбоксы
        document.getElementById('formatCsv').checked = outputFormats.includes('csv');
        document.getElementById('formatXlsx').checked = outputFormats.includes('xlsx');
        document.getElementById('formatJson').checked = outputFormats.includes('json');

        // Тумблеры интеграций
        const destinations = settings.destinations || { json: true, amocrm: false, bitrix: false, '1c': false };
        const byId = (id) => document.getElementById(id);
        if (byId('destJson'))  byId('destJson').checked  = !!destinations.json;
        if (byId('destAmo'))   byId('destAmo').checked   = !!destinations.amocrm;
        if (byId('destBitrix'))byId('destBitrix').checked= !!destinations.bitrix;
        if (byId('dest1C'))    byId('dest1C').checked    = !!destinations['1c'];

        // Показать статус подключения amoCRM
        const amoStatus = document.getElementById('amoStatus');
        if (amoStatus) {
            const connected = !!settings.amocrm_connected;
            amoStatus.textContent = connected ? 'amoCRM: подключено' : 'amoCRM: не подключено';
        }
    }

    async updateSettings() {
        const outputFormats = [];
        if (document.getElementById('formatCsv').checked) outputFormats.push('csv');
        if (document.getElementById('formatXlsx').checked) outputFormats.push('xlsx');
        if (document.getElementById('formatJson').checked) outputFormats.push('json');

        if (outputFormats.length === 0) {
            alert('Выберите хотя бы один формат');
            return;
        }

        // Собираем тумблеры интеграций
        const destinations = {
            json: !!document.getElementById('destJson').checked,
            amocrm: !!document.getElementById('destAmo').checked,
            bitrix: !!document.getElementById('destBitrix').checked,
            '1c': !!document.getElementById('dest1C').checked
        };

        try {
            const response = await fetch('/user/settings', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify({
                    output_formats: outputFormats,
                    destinations: destinations
                })
            });

            if (response.ok) {
                const data = await response.json();
                if (data.status === 'success') {
                    alert('Настройки успешно сохранены');
                } else {
                    alert('Ошибка сохранения настроек: ' + (data.message || 'Неизвестная ошибка'));
                }
            } else {
                alert('Ошибка сохранения настроек');
            }
        } catch (error) {
            console.error('Ошибка сохранения настроек:', error);
            alert('Ошибка сохранения настроек');
        }
    }

    async loadAmoStatus() {
        try {
            const r = await fetch('/user/amocrm/status', { credentials: 'include' });
            if (!r.ok) return;
            const data = await r.json();
            const amoStatus = document.getElementById('amoStatus');
            if (amoStatus && data && data.data) {
                amoStatus.textContent = data.data.connected ? 'amoCRM: подключено' : 'amoCRM: не подключено';
            }
        } catch (e) {
            // no-op
        }
    }

    async loadAmoSettings() {
        try {
            const r = await fetch('/user/integrations/amocrm/settings', { credentials: 'include' });
            if (!r.ok) return;
            const data = await r.json();
            const s = data.data || {};
            document.getElementById('amoDomain').value = s.account_domain || '';
            const creds = s.credentials || {};
            document.getElementById('amoClientId').value = creds.client_id || '';
            document.getElementById('amoClientSecret').value = ''; // не заполняем секрет (маскируется)
            document.getElementById('amoRedirectUri').value = creds.redirect_uri || '';
        } catch (e) {
            console.warn('Не удалось загрузить настройки amoCRM', e);
        }
    }

    async saveAmoSettings() {
        const body = {
            account_domain: document.getElementById('amoDomain').value.trim(),
            client_id: document.getElementById('amoClientId').value.trim(),
            client_secret: document.getElementById('amoClientSecret').value.trim(),
            redirect_uri: document.getElementById('amoRedirectUri').value.trim(),
            enabled: true
        };
        if (!body.account_domain || !body.client_id || !body.client_secret) {
            alert('Введите domain, client_id и client_secret');
            return;
        }
        try {
            const r = await fetch('/user/integrations/amocrm/settings', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify(body)
            });
            const data = await r.json();
            if (r.ok && data.status === 'success') {
                alert('Настройки amoCRM сохранены');
                // очистим поле секрета
                document.getElementById('amoClientSecret').value = '';
                this.loadAmoStatus();
            } else {
                alert('Ошибка сохранения: ' + (data.message || ''));
            }
        } catch (e) {
            alert('Ошибка сети при сохранении настроек');
        }
    }

    async ensureAmoCredentialsThenConnect() {
        try {
            const r = await fetch('/user/integrations/amocrm/settings', { credentials: 'include' });
            if (!r.ok) {
                alert('Сначала заполните настройки amoCRM');
                return;
            }
            const data = await r.json();
            const s = data.data || {};
            const hasDomain = !!(s.account_domain && s.account_domain.length > 0);
            const creds = s.credentials || {};
            const hasClient = !!(creds.client_id && creds.client_id.length > 0);
            // client_secret не возвращаем, поэтому просто проверяем наличие client_id и домена
            if (!hasDomain || !hasClient) {
                alert('Сначала заполните domain и client_id в настройках amoCRM');
                return;
            }
            window.location.href = '/user/amocrm/connect';
        } catch (e) {
            alert('Не удалось проверить настройки amoCRM');
        }
    }

    async loadBitrixSettings() {
        try {
            const r = await fetch('/user/integrations/bitrix/settings', { credentials: 'include' });
            if (!r.ok) return;
            const data = await r.json();
            const s = data.data || {};
            document.getElementById('bitrixWebhookBase').value = s.webhook_base || '';
        } catch (e) {
            console.warn('Не удалось загрузить настройки Bitrix', e);
        }
    }

    async saveBitrixSettings() {
        const webhookBase = document.getElementById('bitrixWebhookBase').value.trim();
        if (!webhookBase) {
            alert('Введите базовый URL вебхука Bitrix');
            return;
        }
        try {
            const r = await fetch('/user/integrations/bitrix/settings', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify({
                    webhook_base: webhookBase,
                    enabled: true
                })
            });
            const data = await r.json();
            if (r.ok && data.status === 'success') {
                alert('Настройки Bitrix сохранены');
            } else {
                alert('Ошибка сохранения Bitrix: ' + (data.message || ''));
            }
        } catch (e) {
            alert('Ошибка сети при сохранении настроек Bitrix');
        }
    }

    async changePassword() {
        const currentPassword = document.getElementById('currentPassword').value;
        const newPassword = document.getElementById('newPassword').value;
        const confirmPassword = document.getElementById('confirmPassword').value;

        if (!currentPassword || !newPassword || !confirmPassword) {
            alert('Все поля обязательны для заполнения');
            return;
        }

        if (newPassword.length < 6) {
            alert('Новый пароль должен содержать минимум 6 символов');
            return;
        }

        if (newPassword !== confirmPassword) {
            alert('Новые пароли не совпадают');
            return;
        }

        try {
            const response = await fetch('/user/change-password', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify({
                    current_password: currentPassword,
                    new_password: newPassword
                })
            });

            if (response.ok) {
                const data = await response.json();
                if (data.status === 'success') {
                    alert('Пароль успешно изменен');
                    // Очищаем поля формы
                    document.getElementById('currentPassword').value = '';
                    document.getElementById('newPassword').value = '';
                    document.getElementById('confirmPassword').value = '';
                } else {
                    alert('Ошибка смены пароля: ' + (data.message || 'Неизвестная ошибка'));
                }
            } else {
                const data = await response.json();
                alert('Ошибка смены пароля: ' + (data.message || 'Неизвестная ошибка'));
            }
        } catch (error) {
            console.error('Ошибка смены пароля:', error);
            alert('Ошибка смены пароля');
        }
    }

    async logout() {
        try {
            const response = await fetch('/user/logout', {
                method: 'POST',
                credentials: 'include'
            });

            if (response.ok) {
                window.location.href = '/static/user/login.html';
            }
        } catch (error) {
            console.error('Ошибка выхода:', error);
            window.location.href = '/static/user/login.html';
        }
    }

    initTheme() {
        // Загружаем сохраненную тему или используем текущую из HTML
        const savedTheme = localStorage.getItem('theme');
        const currentTheme = document.documentElement.getAttribute('data-theme');
        const themeSwitch = document.getElementById('themeSwitch');
        
        // Определяем тему: сначала из localStorage, потом из HTML, потом по умолчанию
        const theme = savedTheme || currentTheme || 'light';
        
        // Устанавливаем тему
        document.documentElement.setAttribute('data-theme', theme);
        
        // Синхронизируем переключатель
        if (themeSwitch) {
            themeSwitch.checked = theme === 'dark';
        }
        
        // Сохраняем тему в localStorage, если её там не было
        if (!savedTheme) {
            localStorage.setItem('theme', theme);
        }
    }

    toggleTheme() {
        const themeSwitch = document.getElementById('themeSwitch');
        const currentTheme = document.documentElement.getAttribute('data-theme');
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
        
        document.documentElement.setAttribute('data-theme', newTheme);
        localStorage.setItem('theme', newTheme);
        
        if (themeSwitch) {
            themeSwitch.checked = newTheme === 'dark';
        }
    }
}

// Initialize dashboard when page loads
document.addEventListener('DOMContentLoaded', () => {
    new UserDashboard();
});
