// Dashboard functionality
class UserDashboard {
    constructor() {
        this.init();
    }

    init() {
        this.bindEvents();
        this.initTheme();
        this.loadUserData();
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
        document.getElementById('userEmail').value = userData.email;
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
            `${subscriptionData.period_start} - ${subscriptionData.period_end}`;
        document.getElementById('subscriptionQuota').textContent = subscriptionData.quota_total || '0';
        document.getElementById('usageCount').textContent = subscriptionData.usage_count || '0';
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
        // Загружаем сохраненную тему
        const savedTheme = localStorage.getItem('theme');
        const themeSwitch = document.getElementById('themeSwitch');
        
        if (savedTheme) {
            document.documentElement.setAttribute('data-theme', savedTheme);
            if (themeSwitch) {
                themeSwitch.checked = savedTheme === 'dark';
            }
        } else {
            // По умолчанию светлая тема
            document.documentElement.setAttribute('data-theme', 'light');
            if (themeSwitch) {
                themeSwitch.checked = false;
            }
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
