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
        document.getElementById('userName').textContent = userData.name || userData.email;
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
                }
            }
        } catch (error) {
            console.error('Ошибка загрузки настроек:', error);
        }
    }

    displaySettings(settings) {
        // Получаем список разрешенных форматов
        const outputFormats = settings.output_formats || ['csv', 'xlsx', 'json'];
        
        // Устанавливаем чекбоксы
        document.getElementById('formatCsv').checked = outputFormats.includes('csv');
        document.getElementById('formatXlsx').checked = outputFormats.includes('xlsx');
        document.getElementById('formatJson').checked = outputFormats.includes('json');
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

        try {
            const response = await fetch('/user/settings', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify({
                    output_formats: outputFormats
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
