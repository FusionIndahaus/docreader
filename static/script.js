
class DocumentAIApp {
    constructor() {
        this.form = document.getElementById('uploadForm');
        this.messageTextarea = document.getElementById('message');
        this.fileInput = document.getElementById('file');
        this.fileNameSpan = document.getElementById('file-name');
        this.submitBtn = document.getElementById('submitBtn');
        this.statusMessage = document.getElementById('statusMessage');
        this.refreshBtn = document.getElementById('refreshBtn');
        this.responseList = document.getElementById('responseList');
        
        this.isUploading = false;
        this.selectedFile = null;
        this.results = [];
        
        this.init();
    }
    
    init() {
        this.setupEventListeners();
        this.loadExistingResults();
        this.connectLiveUpdates();
        this.animateOnLoad();
        this.initTheme();
    }
    
    setupEventListeners() {
        
        this.form.addEventListener('submit', (e) => this.handleFormSubmit(e));
        
        this.fileInput.addEventListener('change', (e) => this.handleFileSelect(e));
        
        this.setupDragAndDrop();
        
        this.refreshBtn.addEventListener('click', () => this.loadExistingResults());
        const themeSwitch = document.getElementById('themeSwitch');
        if (themeSwitch) {
            themeSwitch.addEventListener('change', () => this.toggleTheme());
        }
        // Делегируем клик по кнопке очистки, чтобы работало надёжно
        this.form.addEventListener('click', (e) => {
            const target = e.target.closest('.selected-file-clear');
            if (target && target.id === 'clearFileBtn') {
                e.preventDefault();
                e.stopPropagation();
                this.clearSelectedFile();
            }
        });
        
    }

    initTheme() {
        const saved = localStorage.getItem('theme') || 'light';
        document.documentElement.setAttribute('data-theme', saved);
        const themeSwitch = document.getElementById('themeSwitch');
        if (themeSwitch) {
            themeSwitch.checked = saved === 'dark';
        }
    }

    toggleTheme() {
        const current = document.documentElement.getAttribute('data-theme') || 'light';
        const next = current === 'dark' ? 'light' : 'dark';
        document.documentElement.setAttribute('data-theme', next);
        localStorage.setItem('theme', next);
        const themeSwitch = document.getElementById('themeSwitch');
        if (themeSwitch) {
            themeSwitch.checked = next === 'dark';
        }
    }
    
    connectLiveUpdates() {
        try {
            const evtSource = new EventSource('/events');
            evtSource.onmessage = (event) => {
                try {
                    const payload = JSON.parse(event.data);
                    if (payload && payload.id) {
                        // добавляем или обновляем локальный список
                        const existsIndex = this.results.findIndex(r => r.id === payload.id);
                        if (existsIndex >= 0) {
                            this.results[existsIndex] = payload;
                        } else {
                            this.results.push(payload);
                        }
                        this.renderResults(this.results);
                    }
                } catch (e) {
                    console.error('ERROR: Невалидное SSE сообщение', e);
                }
            };
            evtSource.onerror = () => {
                // Автопереподключение: браузер сам переподключается для EventSource
            };
            this.evtSource = evtSource;
        } catch (e) {
            console.error('ERROR: Не удалось подключиться к событиям', e);
        }
    }

    setupDragAndDrop() {
        const dropZone = this.form;
        
        if (!dropZone) {
            return;
        }
        
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
    
    async handleFormSubmit(e) {
        e.preventDefault();
        
        if (this.isUploading) {
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
        
        const maxSize = 50 * 1024 * 1024;
        if (this.selectedFile.size > maxSize) {
            throw new Error('Файл слишком большой. Максимальный размер: 50 МБ');
        }
        
    }
    
    async uploadDocument() {
        this.isUploading = true;
        this.setLoadingState(true);
        
        try {
            // Прогресс-бар (визуальный, клиентский)
            const progressEl = document.getElementById('uploadBar');
            if (progressEl) {
                progressEl.style.width = '15%';
            }
            const formData = new FormData();
            formData.append('message', document.getElementById('message').value.trim());
            formData.append('file', this.selectedFile);
            const selectedFormat = (document.querySelector('input[name="outputFormat"]:checked')?.value || 'json').toLowerCase();
            formData.append('outputFormat', selectedFormat);
            
            const response = await fetch('/upload', {
                method: 'POST',
                body: formData
            });
            
            if (progressEl) {
                progressEl.style.width = '70%';
            }
            const result = await response.json();
            
            if (!response.ok) {
                throw new Error(result.message || `Ошибка сервера: ${response.status}`);
            }
            
            this.showSuccess(result.message);
            this.clearForm();
            if (progressEl) {
                progressEl.style.width = '100%';
                setTimeout(() => { progressEl.style.width = '0%'; }, 700);
            }
            
            setTimeout(() => this.loadExistingResults(), 2000);
            
        } catch (error) {
            console.error('ERROR: Ошибка загрузки:', error);
            throw error;
        } finally {
            this.isUploading = false;
            this.setLoadingState(false);
        }
    }
    
    handleFileSelect(e) {
        const files = e.target.files;
        if (files.length > 0) {
            this.processSelectedFile(files[0]);
        }
    }
    
    handleFileDrop(e) {
        const files = e.dataTransfer.files;
        if (files.length > 0) {
            this.fileInput.files = files; // обновляем input
            this.processSelectedFile(files[0]);
        }
    }
    
    processSelectedFile(file) {
        const allowedTypes = ['application/pdf', 'image/jpeg', 'image/jpg', 'image/png'];
        const allowedExtensions = ['.pdf', '.jpg', '.jpeg', '.png'];
        const fileExtension = '.' + file.name.split('.').pop().toLowerCase();
        
        if (!allowedTypes.includes(file.type) && !allowedExtensions.includes(fileExtension)) {
            this.showError('Неподдерживаемый тип файла. Разрешены только PDF, JPG и PNG');
            return;
        }
        
        this.selectedFile = file;
        this.updateFilePreview(file);

        // Показать красивую строку выбранного файла
        const selectedRow = document.getElementById('selectedFile');
        const nameEl = document.getElementById('selectedFileName');
        if (selectedRow && nameEl) {
            nameEl.textContent = file.name;
            selectedRow.hidden = false;
        }
    }
    
    updateFilePreview(file) {
        const icon = this.getFileIcon(file.name);
        this.fileNameSpan.textContent = `${icon} ${file.name}`;
        
    }
    
    clearSelectedFile() {
        this.selectedFile = null;
        this.fileInput.value = '';
        this.fileNameSpan.textContent = 'Файл не выбран';
        const selectedRow = document.getElementById('selectedFile');
        const nameEl = document.getElementById('selectedFileName');
        if (nameEl) nameEl.textContent = 'Файл не выбран';
        if (selectedRow) selectedRow.hidden = true;
        this.fileInput.focus();
        
    }
    
    async loadExistingResults() {
        try {
            const response = await fetch('/results');
            
            if (response.ok) {
                const data = await response.json();
                this.results = data.data || [];
                this.renderResults(this.results);
                
            }
            
        } catch (error) {
            console.error('ERROR: Ошибка загрузки результатов:', error);
        }
    }
    
    renderResults(results) {
        if (!results || results.length === 0) {
            this.responseList.innerHTML = `
                <p class="no-data">Ожидание данных...</p>
            `;
            return;
        }
        
        const sortedResults = [...results].sort((a, b) => 
            new Date(b.timestamp) - new Date(a.timestamp)
        );
        
        this.responseList.innerHTML = sortedResults.map(result => `
            <div class="response-item" data-id="${result.id}">
                <div class="result-content">
                    <div class="result-text">${this.formatResultText(result.text)}</div>
                </div>
            </div>
        `).join('');
        
    }
    
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
    
    clearForm() {
        document.getElementById('message').value = '';
        this.clearSelectedFile();
    }
    
    setLoadingState(isLoading) {
        if (isLoading) {
            this.submitBtn.textContent = 'Отправляем...';
            this.submitBtn.disabled = true;
        } else {
            this.submitBtn.textContent = 'Начать обработку';
            this.submitBtn.disabled = false;
        }
    }
    
    showMessage(message, type = 'info') {
        this.statusMessage.className = `status-message status-${type}`;
        this.statusMessage.textContent = message;
        this.statusMessage.style.display = 'block';
        
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
    
    formatResultText(text) {
        return text
            .replace(/\n/g, '<br>')
            .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
            .replace(/\*(.*?)\*/g, '<em>$1</em>');
    }
    
    animateOnLoad() {
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

let app;

document.addEventListener('DOMContentLoaded', () => {
    app = new DocumentAIApp();
});

window.addEventListener('error', (event) => {
    console.error('ERROR: JavaScript Error:', event.error);
    if (app) {
        app.showError('Произошла непредвиденная ошибка. Попробуйте обновить страницу.');
    }
});

window.addEventListener('unhandledrejection', (event) => {
    console.error('ERROR: Unhandled Promise Rejection:', event.reason);
    if (app) {
        app.showError('Ошибка сети или сервера. Проверьте соединение.');
    }
}); 