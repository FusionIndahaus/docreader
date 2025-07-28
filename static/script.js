// Document AI - Frontend JavaScript
// Обработка форм, загрузка файлов, и взаимодействие с API

class DocumentAIApp {
    constructor() {
        // Основные элементы интерфейса
        this.form = document.getElementById('uploadForm');
        this.fileInput = document.getElementById('file');
        this.fileUploadArea = document.getElementById('fileUploadArea');
        this.filePreview = document.getElementById('filePreview');
        this.fileName = document.getElementById('fileName');
        this.fileSize = document.getElementById('fileSize');
        this.fileRemove = document.getElementById('fileRemove');
        this.submitBtn = document.getElementById('submitBtn');
        this.statusMessage = document.getElementById('statusMessage');
        this.refreshBtn = document.getElementById('refreshBtn');
        this.clearBtn = document.getElementById('clearBtn');
        this.resultsContainer = document.getElementById('resultsContainer');
        
        // Состояние приложения
        this.isUploading = false;
        this.selectedFile = null;
        this.results = [];
        
        // Запускаем инициализацию
        this.init();
    }
    
    init() {
        console.log('INFO: Document AI загружается...');
        this.setupEventListeners();
        this.loadExistingResults();
        this.animateOnLoad();
        console.log('INFO: Document AI готов к работе');
    }
    
    // Настройка всех обработчиков событий
    setupEventListeners() {
        // Форма загрузки
        this.form.addEventListener('submit', (e) => this.handleFormSubmit(e));
        
        // Загрузка файлов
        this.fileInput.addEventListener('change', (e) => this.handleFileSelect(e));
        this.fileRemove.addEventListener('click', () => this.clearSelectedFile());
        
        // Drag & Drop для файлов
        this.setupDragAndDrop();
        
        // Кнопки управления
        this.refreshBtn.addEventListener('click', () => this.loadExistingResults());
        this.clearBtn.addEventListener('click', () => this.clearResults());
        
        // Автообновление результатов каждые 30 секунд
        setInterval(() => this.loadExistingResults(), 30000);
    }
    
    // Настройка drag & drop для файлов
    setupDragAndDrop() {
        const dropZone = this.fileUploadArea;
        
        ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
            dropZone.addEventListener(eventName, this.preventDefaults, false);
        });
        
        ['dragenter', 'dragover'].forEach(eventName => {
            dropZone.addEventListener(eventName, () => {
                dropZone.classList.add('drag-over');
            });
        });
        
        ['dragleave', 'drop'].forEach(eventName => {
            dropZone.addEventListener(eventName, () => {
                dropZone.classList.remove('drag-over');
            });
        });
        
        dropZone.addEventListener('drop', (e) => this.handleFileDrop(e));
    }
    
    preventDefaults(e) {
        e.preventDefault();
        e.stopPropagation();
    }
    
    // Обработка отправки формы
    async handleFormSubmit(e) {
        e.preventDefault();
        
        if (this.isUploading) {
            console.log('INFO: Уже идет загрузка, пропускаем...');
            return;
        }
        
        try {
            this.validateForm();
            await this.uploadDocument();
        } catch (error) {
            console.error('ERROR: Ошибка отправки:', error);
            this.showError(error.message);
        }
    }
    
    // Валидация формы
    validateForm() {
        const message = document.getElementById('message').value.trim();
        
        if (!message) {
            throw new Error('Пожалуйста, опишите что вас интересует в документе');
        }
        
        if (message.length < 10) {
            throw new Error('Описание слишком короткое. Напишите подробнее что нужно найти');
        }
        
        if (!this.selectedFile) {
            throw new Error('Выберите файл для обработки');
        }
        
        // Проверяем размер файла (50MB max)
        const maxSize = 50 * 1024 * 1024;
        if (this.selectedFile.size > maxSize) {
            throw new Error('Файл слишком большой. Максимальный размер: 50 МБ');
        }
        
        console.log('INFO: Форма валидна');
    }
    
    // Загрузка документа на сервер
    async uploadDocument() {
        this.isUploading = true;
        this.setLoadingState(true);
        
        try {
            console.log('INFO: Отправляем документ на обработку...');
            
            const formData = new FormData();
            formData.append('message', document.getElementById('message').value.trim());
            formData.append('file', this.selectedFile);
            
            const response = await fetch('/upload', {
                method: 'POST',
                body: formData
            });
            
            const result = await response.json();
            
            if (!response.ok) {
                throw new Error(result.message || `Ошибка сервера: ${response.status}`);
            }
            
            console.log('INFO: Документ успешно отправлен');
            this.showSuccess(result.message);
            this.clearForm();
            
            // Ждем немного и загружаем результаты
            setTimeout(() => this.loadExistingResults(), 2000);
            
        } catch (error) {
            console.error('ERROR: Ошибка загрузки:', error);
            throw error;
        } finally {
            this.isUploading = false;
            this.setLoadingState(false);
        }
    }
    
    // Обработка выбора файла
    handleFileSelect(e) {
        const files = e.target.files;
        if (files.length > 0) {
            this.processSelectedFile(files[0]);
        }
    }
    
    // Обработка drag & drop файла
    handleFileDrop(e) {
        const files = e.dataTransfer.files;
        if (files.length > 0) {
            this.fileInput.files = files; // обновляем input
            this.processSelectedFile(files[0]);
        }
    }
    
    // Обработка выбранного файла
    processSelectedFile(file) {
        // Проверяем тип файла
        const allowedTypes = ['application/pdf', 'image/jpeg', 'image/jpg', 'image/png'];
        const allowedExtensions = ['.pdf', '.jpg', '.jpeg', '.png'];
        const fileExtension = '.' + file.name.split('.').pop().toLowerCase();
        
        if (!allowedTypes.includes(file.type) && !allowedExtensions.includes(fileExtension)) {
            this.showError('Неподдерживаемый тип файла. Разрешены только PDF, JPG и PNG');
            return;
        }
        
        this.selectedFile = file;
        this.updateFilePreview(file);
        console.log(`INFO: Выбран файл: ${file.name} (${this.formatFileSize(file.size)})`);
    }
    
    // Обновление превью файла
    updateFilePreview(file) {
        this.fileName.textContent = file.name;
        this.fileSize.textContent = this.formatFileSize(file.size);
        
        // Показываем превью, скрываем placeholder
        this.filePreview.style.display = 'flex';
        this.fileUploadArea.querySelector('.file-upload-content').style.display = 'none';
        
        // Добавляем иконку в зависимости от типа файла
        const icon = this.getFileIcon(file.name);
        this.fileName.textContent = `${icon} ${file.name}`;
    }
    
    // Очистка выбранного файла
    clearSelectedFile() {
        this.selectedFile = null;
        this.fileInput.value = '';
        
        // Скрываем превью, показываем placeholder
        this.filePreview.style.display = 'none';
        this.fileUploadArea.querySelector('.file-upload-content').style.display = 'block';
        
        console.log('INFO: Файл удален');
    }
    
    // Загрузка существующих результатов
    async loadExistingResults() {
        try {
            console.log('INFO: Загружаем результаты...');
            
            const response = await fetch('/results');
            const data = await response.json();
            
            if (response.ok && data.data) {
                this.results = data.data;
                this.renderResults(this.results);
                
                if (this.results.length > 0) {
                    this.clearBtn.style.display = 'inline-block';
                }
            }
            
        } catch (error) {
            console.error('ERROR: Ошибка загрузки результатов:', error);
        }
    }
    
    // Отрисовка результатов
    renderResults(results) {
        if (!results || results.length === 0) {
            this.resultsContainer.innerHTML = `
                <div class="no-results">
                    <div class="no-results-icon">🤖</div>
                    <h3>Пока результатов нет</h3>
                    <p>Загрузите документ выше, и здесь появятся результаты анализа</p>
                </div>
            `;
            return;
        }
        
        // Сортируем по времени (новые сверху)
        const sortedResults = [...results].sort((a, b) => 
            new Date(b.timestamp) - new Date(a.timestamp)
        );
        
        this.resultsContainer.innerHTML = sortedResults.map(result => `
            <div class="result-item" data-id="${result.id}">
                <div class="result-header">
                    <div class="result-time">${this.formatTime(result.timestamp)}</div>
                    <div class="result-status status-${result.status || 'completed'}">${this.getStatusText(result.status)}</div>
                </div>
                <div class="result-content">
                    <div class="result-text">${this.formatResultText(result.text)}</div>
                </div>
                <div class="result-actions">
                    <button class="copy-btn" onclick="app.copyToClipboard('${result.id}')">Копировать</button>
                </div>
            </div>
        `).join('');
        
        console.log(`INFO: Отображено результатов: ${sortedResults.length}`);
    }
    
    // Очистка всех результатов (только UI)
    clearResults() {
        if (confirm('Очистить все результаты? Это действие нельзя отменить.')) {
            this.results = [];
            this.renderResults([]);
            this.clearBtn.style.display = 'none';
            console.log('INFO: Результаты очищены');
        }
    }
    
    // Копирование результата в буфер обмена
    async copyToClipboard(resultId) {
        const result = this.results.find(r => r.id === resultId);
        if (!result) return;
        
        try {
            await navigator.clipboard.writeText(result.text);
            this.showSuccess('Текст скопирован в буфер обмена');
        } catch (error) {
            console.error('ERROR: Ошибка копирования:', error);
            this.showError('Не удалось скопировать текст');
        }
    }
    
    // Очистка формы после успешной отправки
    clearForm() {
        document.getElementById('message').value = '';
        this.clearSelectedFile();
    }
    
    // Установка состояния загрузки
    setLoadingState(isLoading) {
        const btnText = this.submitBtn.querySelector('.btn-text');
        const btnLoader = this.submitBtn.querySelector('.btn-loader');
        
        if (isLoading) {
            btnText.style.display = 'none';
            btnLoader.style.display = 'inline';
            this.submitBtn.disabled = true;
        } else {
            btnText.style.display = 'inline';
            btnLoader.style.display = 'none';
            this.submitBtn.disabled = false;
        }
    }
    
    // Показ сообщений пользователю
    showMessage(message, type = 'info') {
        this.statusMessage.className = `status-message status-${type}`;
        this.statusMessage.textContent = message;
        this.statusMessage.style.display = 'block';
        
        // Автоскрытие через 5 секунд для успехов и ошибок
        if (type === 'success' || type === 'error') {
            setTimeout(() => {
                this.statusMessage.style.display = 'none';
            }, 5000);
        }
    }
    
    showSuccess(message) {
        this.showMessage(message, 'success');
    }
    
    showError(message) {
        this.showMessage(message, 'error');
    }
    
    showInfo(message) {
        this.showMessage(message, 'info');
    }
    
    // Вспомогательные функции
    
    formatFileSize(bytes) {
        if (bytes === 0) return '0 Б';
        const k = 1024;
        const sizes = ['Б', 'КБ', 'МБ', 'ГБ'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }
    
    getFileIcon(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        const icons = {
            'pdf': '[PDF]',
            'jpg': '[IMG]',
            'jpeg': '[IMG]', 
            'png': '[IMG]'
        };
        return icons[ext] || '[FILE]';
    }
    
    formatTime(timestamp) {
        const date = new Date(timestamp);
        const now = new Date();
        const diffMinutes = Math.floor((now - date) / (1000 * 60));
        
        if (diffMinutes < 1) return 'только что';
        if (diffMinutes < 60) return `${diffMinutes} мин назад`;
        if (diffMinutes < 1440) return `${Math.floor(diffMinutes / 60)} ч назад`;
        
        return date.toLocaleDateString('ru-RU', {
            day: 'numeric',
            month: 'short',
            hour: '2-digit',
            minute: '2-digit'
        });
    }
    
    getStatusText(status) {
        const statusTexts = {
            'completed': 'Готово',
            'processing': 'Обработка',
            'error': 'Ошибка',
            'pending': 'Ожидание'
        };
        return statusTexts[status] || 'Готово';
    }
    
    formatResultText(text) {
        // Простое форматирование: добавляем переносы строк и выделяем важные части
        return text
            .replace(/\n/g, '<br>')
            .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
            .replace(/\*(.*?)\*/g, '<em>$1</em>');
    }
    
    // Анимации при загрузке
    animateOnLoad() {
        // Простая анимация появления элементов
        const animatedElements = document.querySelectorAll('.card, .main-header');
        animatedElements.forEach((el, index) => {
            el.style.opacity = '0';
            el.style.transform = 'translateY(20px)';
            
            setTimeout(() => {
                el.style.transition = 'opacity 0.6s ease, transform 0.6s ease';
                el.style.opacity = '1';
                el.style.transform = 'translateY(0)';
            }, index * 100);
        });
    }
}

// Глобальная переменная для доступа из HTML (для кнопок копирования)
let app;

// Запускаем приложение когда DOM готов
document.addEventListener('DOMContentLoaded', () => {
    app = new DocumentAIApp();
});

// Обработка ошибок JavaScript
window.addEventListener('error', (event) => {
    console.error('ERROR: JavaScript Error:', event.error);
    if (app) {
        app.showError('Произошла непредвиденная ошибка. Попробуйте обновить страницу.');
    }
});

// Обработка отклоненных промисов
window.addEventListener('unhandledrejection', (event) => {
    console.error('ERROR: Unhandled Promise Rejection:', event.reason);
    if (app) {
        app.showError('Ошибка сети или сервера. Проверьте соединение.');
    }
}); 