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
        this.selectedFiles = [];
        this.results = [];
        this.activeBatch = null; // { id, expected, format, received, downloads: [], responses: [] }
        
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
            // Разбиваем на лету по запятым/точкам с запятой
            this.columnsInput.addEventListener('input', (e) => {
                this.handleColumnsTyping(e);
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
                        const isActiveBatch = this.activeBatch && payload.batchId && payload.batchId === this.activeBatch.id;
                        if (isActiveBatch) {
                            this.activeBatch.received += 1;
                            if (payload.download) this.activeBatch.downloads.push(payload.download);
                            this.activeBatch.responses.push(payload);
                            if (this.activeBatch.received >= this.activeBatch.expected) {
                                const batch = this.activeBatch;
                                this.activeBatch = null;
                                this.handleBatchCompleted(batch).catch((e) => console.error('ERROR: batch finalize:', e));
                            }
                        } else {
                            // Не скачиваем автоматически для чужих/прошлых событий
                            const existsIndex = this.results.findIndex(r => r.id === payload.id);
                            if (existsIndex >= 0) {
                                this.results[existsIndex] = payload;
                            } else {
                                this.results.push(payload);
                            }
                            this.renderResults(this.results);
                        }
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
        
        if (!this.selectedFiles || this.selectedFiles.length === 0) {
            throw new Error('Выберите один или несколько файлов для обработки');
        }
        
        const maxSize = 50 * 1024 * 1024;
        for (const f of this.selectedFiles) {
            if (f.size > maxSize) {
                throw new Error(`Файл ${f.name} слишком большой. Максимальный размер: 50 МБ`);
            }
        }
        
    }
    
    async uploadDocument() {
        this.isUploading = true;
        this.setLoadingState(true);
        
        try {
            const selectedFormat = (document.querySelector('input[name="outputFormat"]:checked')?.value || 'csv').toLowerCase();
            // Сохраним последние колонки локально для README
            try { localStorage.setItem('columns1c:last', JSON.stringify(this.columns || [])); } catch {}

            const batchId = this.generateBatchId();
            this.activeBatch = {
                id: batchId,
                expected: this.selectedFiles.length,
                format: selectedFormat,
                received: 0,
                downloads: [],
                responses: []
            };

            await this.uploadBatchSequentially(batchId, this.selectedFiles, selectedFormat, (this.columns || []).join(','));
            // Завершение произойдет по SSE в handleBatchCompleted
            
        } catch (error) {
            console.error('ERROR: Ошибка загрузки:', error);
            throw error;
        } finally {
            this.isUploading = false;
            this.setLoadingState(false);
        }
    }

    async uploadBatchSequentially(batchId, files, selectedFormat, columns1cStr) {
        for (let i = 0; i < files.length; i++) {
            const file = files[i];
            const formData = new FormData();
            formData.append('message', document.getElementById('message').value.trim());
            formData.append('file', file);
            formData.append('outputFormat', selectedFormat);
            formData.append('columns1c', columns1cStr);
            formData.append('batchId', batchId);
            formData.append('seq', String(i + 1));

            const response = await fetch('/upload', { method: 'POST', body: formData });
            const result = await response.json().catch(() => ({}));
            if (!response.ok) {
                throw new Error(result?.message || `Ошибка сервера: ${response.status}`);
            }
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
    
    async handleBatchCompleted(batch) {
        try {
            // Результат показываем только после завершения батча
            if (batch.format === 'csv' && batch.downloads.length > 0) {
                const merged = await this.mergeCsvDownloads(batch.downloads);
                const outName = `merged_${batch.id}.csv`;
                this.downloadTextAsFile(merged, outName);
                this.triggerReadmeDownloadOnce(batch.id);
                this.showSuccess(`Готово: объединено ${batch.downloads.length} CSV-файлов`);
            } else {
                for (const url of batch.downloads) {
                    this.triggerDownload(url);
                }
                this.triggerReadmeDownloadOnce(batch.id);
                this.showSuccess(`Готово: обработано ${batch.downloads.length} файл(ов)`);
            }

            const summary = {
                id: `batch_${batch.id}`,
                text: `Пакет обработан. Файлов: ${batch.expected}. Формат: ${batch.format.toUpperCase()}.`,
                timestamp: new Date().toISOString(),
                status: 'completed'
            };
            this.results.push(summary);
            this.renderResults(this.results);

            this.clearForm();
            setTimeout(() => this.loadExistingResults(), 1000);
        } catch (e) {
            this.showError('Ошибка при объединении результатов');
            throw e;
        }
    }

    async mergeCsvDownloads(urls) {
        const texts = [];
        for (const url of urls) {
            const resp = await fetch(url);
            const text = await resp.text();
            texts.push(text);
        }
        return this.mergeCsvTexts(texts);
    }

    mergeCsvTexts(csvTexts) {
        const userColumns = (this.columns || []).map((c) => String(c || '').trim()).filter((c) => c.length > 0);
        let headerOut = null;
        const dataLines = [];

        for (let i = 0; i < csvTexts.length; i++) {
            const raw = (csvTexts[i] || '').replace(/^\uFEFF/, '');
            const lines = raw.split(/\r?\n/).filter(l => l.length > 0);
            if (lines.length === 0) continue;

            // Инициализируем шапку
            if (!headerOut) {
                if (userColumns.length > 0) {
                    headerOut = userColumns.map(this.escapeCsv).join(',');
                } else {
                    headerOut = lines[0];
                }
            }

            for (let li = 1; li < lines.length; li++) {
                dataLines.push(lines[li]);
            }
        }

        if (!headerOut) return dataLines.join('\n');
        return [headerOut, ...dataLines].join('\n');
    }

    // Разбивает CSV-строку по запятым с учётом кавычек
    splitCsvLine(line) {
        const result = [];
        let current = '';
        let inQuotes = false;
        for (let i = 0; i < line.length; i++) {
            const ch = line[i];
            if (ch === '"') {
                if (inQuotes && line[i + 1] === '"') { // удвоенная кавычка
                    current += '"';
                    i++;
                } else {
                    inQuotes = !inQuotes;
                }
            } else if (ch === ',' && !inQuotes) {
                result.push(current);
                current = '';
            } else {
                current += ch;
            }
        }
        result.push(current);
        return result;
    }

    // Экранирует значение для CSV (кавычки/запятые/переводы строк)
    escapeCsv(value) {
        const v = String(value);
        if (/[",\n\r]/.test(v)) {
            return '"' + v.replace(/"/g, '""') + '"';
        }
        return v;
    }

    generateBatchId() {
        const rnd = Math.random().toString(36).slice(2, 8);
        return `b${Date.now()}_${rnd}`;
    }
    
    triggerDownload(url) {
        try {
            const a = document.createElement('a');
            a.href = this.normalizeUrlForCurrentOrigin(url);
            a.download = '';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
        } catch (e) {
            console.warn('WARN: Не удалось инициировать автоскачивание', e);
        }
    }

    normalizeUrlForCurrentOrigin(url) {
        try {
            if (!url) return url;
            // Относительные URL оставляем как есть
            if (url.startsWith('/')) return url;
            const u = new URL(url, window.location.href);
            // Если хост совпадает — используем текущий протокол/хост/порт
            if (u.host === window.location.host) {
                u.protocol = window.location.protocol;
                return u.toString();
            }
            return url;
        } catch {
            return url;
        }
    }
    
    handleFileSelect(e) {
        const files = Array.from(e.target.files || []);
        if (files.length > 0) {
            this.processSelectedFiles(files);
        } else {
            this.clearSelectedFile();
        }
    }
    
    handleFileDrop(e) {
        const files = Array.from(e.dataTransfer.files || []);
        if (files.length > 0) {
            const dt = new DataTransfer();
            for (const f of files) dt.items.add(f);
            this.fileInput.files = dt.files;
            this.processSelectedFiles(files);
        }
    }
    
    processSelectedFiles(files) {
        const allowedTypes = ['application/pdf', 'image/jpeg', 'image/jpg', 'image/png'];
        const allowedExtensions = ['.pdf', '.jpg', '.jpeg', '.png'];
        const valid = [];
        for (const f of files) {
            const ext = '.' + f.name.split('.').pop().toLowerCase();
            if (allowedTypes.includes(f.type) || allowedExtensions.includes(ext)) {
                valid.push(f);
            }
        }
        if (valid.length === 0) {
            this.showError('Неподдерживаемый тип файла. Разрешены только PDF, JPG и PNG');
            return;
        }
        this.selectedFiles = valid;
        this.updateFilePreview(valid[0], valid.length);

        const selectedRow = document.getElementById('selectedFile');
        const nameEl = document.getElementById('selectedFileName');
        if (selectedRow && nameEl) {
            nameEl.textContent = valid.length === 1 ? valid[0].name : `${valid.length} файла(ов) выбрано`;
            selectedRow.hidden = false;
        }
    }
    
    updateFilePreview(file, count = 1) {
        const icon = this.getFileIcon(file.name);
        this.fileNameSpan.textContent = count === 1 ? `${icon} ${file.name}` : `${count} файлов выбрано`;
        
    }
    
    clearSelectedFile() {
        this.selectedFiles = [];
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

    // На лету: создаём чипы, когда пользователь вводит запятую/точку с запятой
    handleColumnsTyping() {
        if (!this.columnsInput) return;
        const raw = this.columnsInput.value || '';
        if (!raw) return;
        const endsWithSep = /[,;]\s*$/.test(raw);
        const tokens = raw.split(/[,;]+/).map(s => this.normalizeColumn(s)).filter(Boolean);
        if (tokens.length === 0) return;
        // Если нет завершающего разделителя — последний фрагмент считаем незавершённым
        let remainder = '';
        let toAdd = tokens;
        if (!endsWithSep) {
            remainder = toAdd.pop() || '';
        }
        if (toAdd.length > 0) {
            toAdd.forEach((t) => this.addColumn(t));
        }
        this.columnsInput.value = remainder;
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