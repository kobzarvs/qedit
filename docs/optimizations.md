# Huge File Optimizations Catalog

## Применённые

| Оптимизация                               | Описание                                                                                                          |
|-------------------------------------------|-------------------------------------------------------------------------------------------------------------------|
| Memory-Mapped I/O (mmap)                  | Файл проецируется в виртуальную память ОС — чтение без syscall seek+read, zero-copy доступ к любому смещению      |
| SIMD-Accelerated Byte Scanning            | `bytes.IndexByte`/`bytes.Count` используют SSE2/AVX2/NEON инструкции для поиска `\n` со скоростью машинного слова |
| Parallel Newline Counting                 | Данные >16 MB разбиваются на чанки по числу CPU, каждое ядро считает `\n` параллельно через `bytes.Count`         |
| Deferred Initial Indexing                 | Индексируются первые 2 MB (initial sample), управление возвращается мгновенно, полный индекс строится в фоне      |
| Sparse Checkpoint Index                   | Каждая 1024-я строка запоминается как checkpoint — O(log n) поиск произвольной строки через binary search         |
| Byte Anchor Index                         | Якоря с шагом 64KB–4MB по байтовому смещению для O(log n) позиционирования при скролле                            |
| Page Anchor Index                         | Якоря хранят offset + lineStart — позволяют начать чтение с середины строки для длинных строк                     |
| Four-Tier LRU Cache                       | 4 независимых LRU-кеша: line spans (4096), decoded lines (256), line info (256), page data (8)                    |
| Batch Lock Acquisition                    | Checkpoint'ы, byte/page anchors, line spans накапливаются в локальном буфере и вставляются за 1 lock              |
| Multi-Line Page Read                      | Один read на целый page-блок (десятки строк) вместо отдельного I/O на каждую строку                               |
| Dual Read Path (mmap/reader)              | `readBytesAt` прозрачно использует mmap slice или fallback seek+read — единый API для всех вызывающих             |
| Persistent Index Cache                    | JSON-кеш checkpoint'ов и якорей на диске — повторное открытие файла без переиндексации                            |
| ASCII Fast-Path Analysis                  | Для ASCII-строк нет вызова `utf8.DecodeRune` — побайтовый проход O(n) с единичным инкрементом                     |
| Synchronous mmap WarmLines                | При наличии mmap prefetch выполняется синхронно без горутины и открытия файлового дескриптора                     |
| In-Flight Deduplication                   | `warmInFlight` map предотвращает запуск дублирующих горутин для одного 64-строчного блока                         |
| Viewport Segment Fetch                    | Для ASCII без табов декодируются только видимые колонки `[scrollX, scrollX+width]`, не вся строка                 |
| Deferred First Paint                      | Первый кадр рисуется без I/O (cache-only) через `lineForDisplayQuick` — мгновенное появление окна                 |
| Cache-Only Quick Read                     | `TryCachedLine` проверяет только кеш без блокировки и I/O — возвращает `(nil, false)` при промахе                 |
| Highlight Span Clipping                   | Binary search обрезает highlight-спаны до видимого диапазона колонок перед render-циклом                          |
| Monotonic Highlight Walker                | `kindAt(idx)` продвигает указатель только вперёд — O(1) амортизированно на символ                                 |
| Column Overscan (±1024)                   | Кешируется расширенный диапазон колонок для highlights — горизонтальный скролл без re-query tree-sitter           |
| Ultra-Long Line Skip                      | Строки >128K rune пропускаются в prefetch — не блокируют рендер viewport'а                                        |
| Per-Line Tree-Sitter Query Split          | Строки >4096 байт запрашиваются отдельно с byte-column clipping — tree-sitter не обходит весь AST                 |
| Per-Line ASCII Flag (tree-sitter)         | Для ASCII-строк колонки = байты — нет byte→rune конвертации в `queryHighlightsInto`                               |
| Patch-List Overlay                        | Правки хранятся как список патчей поверх неизменного base-буфера — нет копии гигабайтного файла                   |
| Sequential Resolve Cache                  | Кеш последнего разрешённого диапазона строк — O(1) для последовательного доступа (render viewport)                |
| Lazy Tab Expansion                        | Табы раскрываются лениво только для видимого окна колонок, без полного `[]rune` decode всей строки                |
| Zero-Copy ASCII Rune Decode               | Прямая конвертация `byte→rune` в 1 проход без промежуточной аллокации `string` для ASCII-строк                    |
| O(1) LRU via Doubly-Linked List           | Замена линейного скана `touchCacheLocked` (до 4096 элементов) на `container/list` — O(1) touch                    |
| Sorted Span Anchor Index                  | Отсортированный вторичный индекс для `nearestCachedSpanScanAnchor` — O(log n) вместо O(n) полного обхода map      |
| Partial ASCII Segment for Non-ASCII Lines | Если non-ASCII символы за пределами viewport — слайс как ASCII до первого multi-byte байта                        |
| Immutable mmap Page Reference             | CR-stripping при анализе вместо мутации stored slice — убирает `copy` mmap данных в pageData                      |
| sync.Pool for Reader Buffers              | Переиспользование `[]byte` буферов через `sync.Pool` вместо `make` на каждый `readBytesAt` (fallback path)        |
| Flattened Priority Span Array             | Pre-sort highlight spans по приоритету при clipping — O(1) вместо O(depth) в `kindAt` для вложенных спанов        |

