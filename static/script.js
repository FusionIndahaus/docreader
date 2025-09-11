
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
        
        // Колонки 1С
        this.columnsInput = document.getElementById('columns1cInput');
        this.columnsChips = document.getElementById('columns1cChips');
        this.columnsHidden = document.getElementById('columns1c');
        this.columns = [];
        
        this.isUploading = false;
        this.selectedFile = null;
        this.results = [];
        
        this.init();
    }
    
    init() {
        this.initTheme();
        this.setupEventListeners();
        this.loadExistingResults();
        this.animateOnLoad();
        this.connectLiveUpdates();
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
        
        // Обработка ввода колонок 1С
        if (this.columnsInput) {
            this.columnsInput.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') {
                    e.preventDefault();
                    this.tryAddColumnsFromInput();
                }
            });
            this.columnsInput.addEventListener('blur', () => {
                // Добавим остаток при уходе фокуса
                this.tryAddColumnsFromInput();
            });
        }
        if (this.columnsChips) {
            this.columnsChips.addEventListener('click', (e) => {
                const btn = e.target.closest('.chip-remove');
                if (!btn) return;
                const value = btn.getAttribute('data-value');
                this.removeColumn(value);
            });
        }
        
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
                        // Автоскачивание, если есть ссылка
                        if (payload.download) {
                            this.triggerDownload(payload.download);
                            // После основного файла — скачиваем README с инструкцией 1С
                            this.triggerReadmeDownloadOnce(payload.id);
                        }
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

    // Не скачивать README несколько раз для одного результата
    triggerReadmeDownloadOnce(id) {
        if (!this._processedReadmeIds) {
            this._processedReadmeIds = new Set();
        }
        if (this._processedReadmeIds.has(id)) return;
        this._processedReadmeIds.add(id);
        this.downloadReadmeForLatestColumns();
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
            const formData = new FormData();
            formData.append('message', document.getElementById('message').value.trim());
            formData.append('file', this.selectedFile);
            // Добавляем выбранный формат результата
            const selectedFormat = (document.querySelector('input[name="outputFormat"]:checked')?.value || 'csv').toLowerCase();
            formData.append('outputFormat', selectedFormat);
            // Добавляем колонки 1С
            formData.append('columns1c', (this.columns || []).join(','));
            // Сохраним последние колонки локально для README
            try { localStorage.setItem('columns1c:last', JSON.stringify(this.columns || [])); } catch {}
            
            const response = await fetch('/upload', {
                method: 'POST',
                body: formData
            });
            
            const result = await response.json();
            
            if (!response.ok) {
                throw new Error(result.message || `Ошибка сервера: ${response.status}`);
            }
            
            // Если сервер вернул download-ссылку — запускаем скачивание
            const dl = result?.data?.download;
            if (dl) {
                this.triggerDownload(dl);
                // README с инструкцией после основного файла
                this.downloadReadmeForLatestColumns();
            }
            
            this.showSuccess(result.message);
            this.clearForm();
            setTimeout(() => this.loadExistingResults(), 1000);
            
        } catch (error) {
            console.error('ERROR: Ошибка загрузки:', error);
            throw error;
        } finally {
            this.isUploading = false;
            this.setLoadingState(false);
        }
    }

    downloadReadmeForLatestColumns() {
        let columns = this.columns;
        try {
            if ((!columns || columns.length === 0)) {
                columns = JSON.parse(localStorage.getItem('columns1c:last') || '[]');
            }
        } catch {}
        const text = this.generateReadmeText(columns || []);
        this.downloadTextAsFile(text, 'README_1C.txt');
    }

    generateReadmeText(columns) {
        const normalized = (columns || [])
            .map((c) => String(c || '').trim())
            .filter((c) => c.length > 0);
        const assignments = normalized.map((col, idx) => {
            // Формируем безопасный идентификатор свойства 1С (пробелы -> без пробелов)
            const prop = this.to1CIdentifier(col);
            return `      НоваяСтрока.${prop} = МассивСлов[${idx}];`;
        }).join('\n');

        const header = `Инструкция по созданию кнопки открытия CSV файла в 1С\n\n` +
        `1) Зайдите в конфигуратор 1С\n` +
        `2) Перейдите на форму, где требуется импорт\n` +
        `3) Создайте кнопки "прочитать файл" и "записать данные"\n` +
        `4) На кнопке "прочитать файл": выберите действие "Прочитать файл"\n` +
        `5) Вставьте следующий код:`;

        const code = `\n\n&НаКлиенте\nПроцедура ПутьКФайлуНачалоВыбора(Элемент, ДанныеВыбора, ВыборДобавлением, СтандартнаяОбработка)\n  Проводник = Новый ДиалогВыбораФайла(РежимДиалогаВыбораФайла.Открытие);\n  Проводник.Заголовок = "Выберите файл с компьютера";\n  \n  Если Объект.ФорматФайла = "CSV" Тогда\n    Фильтр = "CSV Файл|*.csv";\n  ИначеЕсли Объект.ФорматФайла = "XLSX" Тогда\n    Фильтр = "XLSX файл|*.xlsx";\n  Иначе\n    Возврат;\n  КонецЕсли;\n  \n  Проводник.Фильтр = Фильтр;\n  \n  Оповещение = Новый ОписаниеОповещения("ПослеВыбораФайла", ЭтотОбъект);\n  Проводник.Показать(Оповещение);  \nКонецПроцедуры\n\n&НаКлиенте\nПроцедура ПослеВыбораФайла(ВыбранныеФайлы, ДополнительныеПараметры) Экспорт\n  Если ВыбранныеФайлы = неопределено Тогда\n    Возврат;\n  КонецЕсли;\n  \n  Объект.ПутьКФайлу = ВыбранныеФайлы[0];\n  \nКонецПроцедуры\n\n\n&НаКлиенте\nПроцедура ПрочитатьФайл(Команда)\n  Объект.ДанныеФайла.Очистить();\n  \n  Если Объект.ФорматФайла = "CSV" Тогда\n    ПрочитатьФайлCSV();  \n  КонецЕсли;\nКонецПроцедуры \n\n &НаКлиенте\nПроцедура ПрочитатьФайлCSV()\n ПоследовательноеЧтение = Истина;\n  Если ПоследовательноеЧтение Тогда\n    \n    Текст = Новый ЧтениеТекста;\n    Текст.Открыть(Объект.ПутьКФайлу);\n    \n    ТекСтрока = Текст.ПрочитатьСтроку();\n    Пока ТекСтрока <> Неопределено Цикл\n      \n      МассивСлов = СтрРазделить(ТекСтрока, ",");\n      Если МассивСлов.Количество() < ${Math.max(1, normalized.length)} Тогда\n        Продолжить;\n      КонецЕсли;\n      \n      НоваяСтрока = Объект.ДанныеФайла.Добавить();\n${assignments || '      // Добавьте присвоения полей в соответствии с вашими колонками'}\n      \n      ТекСтрока = Текст.ПрочитатьСтроку();\n      \n    КонецЦикла;\n  Иначе\n    Текст = Новый ТекстовыйДокумент;\n    Текст.Прочитать(Объект.ПутьКФайлу);\n    Для НомерСтроки=1 По Текст.КоличествоСтрок() Цикл\n      \n      ТекСтрока = Текст.ПолучитьСтроку(НомерСтроки);\n      МассивСлов = СтрРазделить(ТекСтрока, ",");\n      Если МассивСлов.Количество() < ${Math.max(1, normalized.length)} Тогда\n        Продолжить;\n      КонецЕсли;\n      \n      НоваяСтрока = Объект.ДанныеФайла.Добавить();\n${assignments || '      // Добавьте присвоения полей в соответствии с вашими колонками'}\n      \n    КонецЦикла;\n  КонецЕсли;\nКонецПроцедуры`;

        return header + code + '\n';
    }

    to1CIdentifier(name) {
        // Убираем недопустимые символы, делаем ПаскальКейс
        const cleaned = String(name).replace(/[^\p{L}\p{N}_\s]/gu, ' ').trim();
        if (!cleaned) return 'Поле';
        const parts = cleaned.split(/\s+/).map(p => p.charAt(0).toUpperCase() + p.slice(1));
        return parts.join('');
    }

    downloadTextAsFile(text, fileName) {
        try {
            const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = fileName;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
        } catch (e) {
            console.warn('WARN: Не удалось скачать README', e);
        }
    }
    
    triggerDownload(url) {
        try {
            const a = document.createElement('a');
            a.href = url;
            a.download = '';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
        } catch (e) {
            console.warn('WARN: Не удалось инициировать автоскачивание', e);
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
    
    clearForm() {
        document.getElementById('message').value = '';
        this.clearSelectedFile();
        // Очистка колонок 1С
        this.columns = [];
        if (this.columnsChips) this.columnsChips.innerHTML = '';
        if (this.columnsHidden) this.columnsHidden.value = '';
        if (this.columnsInput) this.columnsInput.value = '';
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

    // ===== Колонки 1С =====
    tryAddColumnsFromInput() {
        if (!this.columnsInput) return;
        const raw = this.columnsInput.value;
        if (!raw) return;
        const parts = raw.split(/[,;\n]+/).map(s => this.normalizeColumn(s)).filter(Boolean);
        if (parts.length === 0) return;
        parts.forEach(p => this.addColumn(p));
        this.columnsInput.value = '';
    }

    normalizeColumn(value) {
        if (!value) return '';
        const trimmed = value.trim();
        if (!trimmed) return '';
        // Ограничим длину и уберём лишние пробелы внутри
        const compact = trimmed.replace(/\s+/g, ' ');
        return compact.slice(0, 64);
    }

    addColumn(value) {
        if (!value) return;
        // без дубликатов (регистр не учитываем)
        const exists = this.columns.find(c => c.toLowerCase() === value.toLowerCase());
        if (exists) return;
        this.columns.push(value);
        this.renderColumnChip(value);
        this.syncHiddenColumns();
    }

    removeColumn(value) {
        this.columns = this.columns.filter(c => c.toLowerCase() !== String(value || '').toLowerCase());
        // убрать чип из DOM
        const chip = this.columnsChips?.querySelector(`.chip[data-value="${CSS.escape(String(value))}"]`);
        if (chip && chip.parentElement) {
            chip.parentElement.removeChild(chip);
        }
        this.syncHiddenColumns();
    }

    renderColumnChip(value) {
        if (!this.columnsChips) return;
        const chip = document.createElement('div');
        chip.className = 'chip';
        chip.setAttribute('data-value', value);
        chip.innerHTML = `
            <span class="chip-label">${this.escapeHTML(value)}</span>
            <button type="button" class="chip-remove" data-value="${this.escapeAttr(value)}" aria-label="Убрать ${this.escapeAttr(value)}">✕</button>
        `;
        this.columnsChips.appendChild(chip);
    }

    syncHiddenColumns() {
        if (this.columnsHidden) {
            this.columnsHidden.value = (this.columns || []).join(',');
        }
    }

    escapeHTML(str) {
        return String(str).replace(/[&<>"']/g, (m) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;','\'':'&#39;'}[m]));
    }

    escapeAttr(str) {
        return this.escapeHTML(str);
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