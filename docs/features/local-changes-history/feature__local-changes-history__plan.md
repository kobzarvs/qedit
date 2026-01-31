# План: Local History - история локальных изменений

## Цель
Показывать все файлы которые изменились в проекте (включая изменения от AI агентов), с inline diff и возможностью restore.

**Референс:** Zed's "Uncommitted Changes" view

**Ключевой use case:** AI агенты (Claude Code, Cursor, Aider) постоянно вносят правки во множество файлов - редактор должен видеть ВСЕ эти изменения.

### Режимы работы

1. **Project Mode** (основной)
   - Папка проекта = CWD (откуда запущен редактор)
   - В git: `git status` показывает все изменённые файлы
   - Вне git: **Project Baseline** — манифест при старте для обнаружения AI-изменений

2. **Single File Mode** (для `sudo qedit /etc/hosts`)
   - Файл вне CWD или запуск с абсолютным путём
   - Отслеживаем только этот один файл
   - Snapshot в `~/.local/state/qedit/files/<hash>`

---

## UI: Changes Panel (как в Zed)

```
┌─ Local Changes [Git] ────────┬─────────────────────────────────────────┐
│ 3 changed files              │ editor.go                               │
│                              │                                         │
│ ● editor.go       [S][M]     │   245 │    if path == "" {              │
│   config.go          [M]     │   246 │        return errors.New()      │
│   newfile.go         [?]     │ - 247 │    data := e.content()          │
│                              │ + 247 │    data := joinLines(e.lines)   │
│                              │ + 248 │    // validation added          │
│                              │   249 │    return os.WriteFile()        │
│                              │                                         │
└──────────────────────────────┴─────────────────────────────────────────┘
 ↑↓: files | Shift+↑↓: diff | r: restore | u: undo | Shift+R: discard all
─────────────────────────────────────────────────────────────────────────
 Restored editor.go. Press 'u' to undo.
```

**Индикаторы режима в заголовке:**
- `[Git]` — видим все изменения в репозитории (staged/unstaged)
- `[Project]` — non-git проект с манифестом (видим все изменения)
- `[File]` — Single File Mode (только один файл)

**Отображение staged/unstaged в Git:**
```
● editor.go       [S][M]  ← staged + unstaged (оба)
  config.go          [M]  ← только unstaged
  newfile.go      [S]     ← только staged (добавлен в index)
  temp.txt           [?]  ← untracked (не в git)
  deleted.go      [S][D]  ← staged deletion
```

**Цветовая схема:**
- `[S]` — зелёный (staged, готово к коммиту)
- `[M]` — жёлтый (modified, unstaged)
- `[?]` — серый (untracked)
- `[D]` — красный (deleted)

**Семантика restore в Git (зависит от состояния файла):**

| Состояние файла | `r` (restore) | Команда Git |
|-----------------|---------------|-------------|
| Только unstaged | Discard unstaged | `git restore --worktree <file>` |
| Только staged | Unstage (вернуть в worktree) | `git restore --staged <file>` |
| Staged + unstaged | Discard unstaged (staged сохраняются!) | `git restore --worktree <file>` |
| Untracked | Удалить файл (с undo) | `rm` + сохранить в undo-буфер |

**`Shift+R` — Discard ALL (опасно!):**
- `git restore --source=HEAD --staged --worktree <file>`
- Сбрасывает ВСЁ (и staged и unstaged) к HEAD
- **Требует подтверждения** если есть staged изменения

### Элементы UI
- **Заголовок**: индикатор режима `[Git]` или `[Active Files Only]`
- **Левая панель**: список файлов с локальными изменениями
- **Правая панель**: inline diff **только выбранного** файла (lazy loading)
- **Статус-бар**: toast-сообщения ("Restored. Press 'u' to undo")
- **Номера строк** и контекст (3 строки вокруг изменений)

### Производительность
- Список файлов (`git status`) обновляется часто
- Diff вычисляется **только для выбранного файла** при навигации
- Для активного буфера diff строится из in-memory содержимого

---

## Логика определения изменений (гибридный подход)

### В git репозитории (основной сценарий)
```
Baseline = HEAD + Index (двухслойная модель)
Список файлов = git status --porcelain=v2 -z
Diff = git diff (staged) + git diff HEAD (unstaged)
Restore = git restore (см. таблицу семантики выше)
```
- **Показывает ВСЕ изменённые файлы** в репозитории
- Различает **staged** и **unstaged** изменения
- Включает изменения от AI агентов (они тоже меняют файлы на диске)
- Автоматически обновляется при любых изменениях

### Вне git репозитория (Project Baseline)

**Проблема lazy snapshot:** AI-агенты меняют файлы в фоне. Если пользователь не открывал `utils.go`, он не увидит изменений от AI.

**Решение: Project Baseline при старте**
```
При открытии проекта (non-git):
  1. Создать легковесный манифест: path → {size, mtime, hash}
  2. Учитывать .gitignore (если есть) для исключений
  3. Пропускать: бинарные, >1MB, node_modules, vendor, etc.

Хранение: ~/.local/state/qedit/projects/<project-hash>/
  - index.json — манифест
  - baseline/ — полные копии только для ИЗМЕНЁННЫХ файлов (lazy)

При запросе :changes:
  - Быстрый скан: сравнить size+mtime с манифестом
  - Для кандидатов: проверить hash
  - Показать ВСЕ изменившиеся файлы (включая от AI)
```

**Исключения (ВСЕГДА, без вопросов):**
- `node_modules/`, `vendor/`, `.git/`, `target/`, `dist/`, `build/`
- Файлы из `.gitignore` (если есть)
- Бинарные файлы, файлы >1MB

Если пользователь открывает файл из исключённой директории — он обрабатывается как **Single File Mode** (вне проекта).

### Single File Mode (sudo qedit /etc/hosts)
```
Условие: файл вне CWD или запуск с абсолютным путём к файлу вне CWD
Baseline = snapshot при открытии
Хранение: ~/.local/state/qedit/files/<file-path-hash>
Restore = восстановить из snapshot
```

**Обработка sudo:** см. `getStateDir()` в разделе "Архитектура хранения".

**⚠️ Git + sudo = проблема safe.directory:**
Git отказывается работать с репозиторием "чужого владельца" (CVE-2022-24765).
Если редактор запущен как root в пользовательском репо — git-команды упадут.

**Рекомендация: sudoedit-подход (в будущем)**
Не запускать редактор как root. Вместо этого:
1. Редактор всегда работает как обычный пользователь
2. При сохранении привилегированного файла — поднимать права только на запись
3. Это безопаснее и избегает проблем с git/ownership

### Обработка внешних изменений (без попапов!)
```
Внешнее изменение файла:
  → НЕ показываем прерывающий popup (раздражает при работе с AI)
  → Показываем индикатор в статус-баре: [Disk Changed]
  → При переключении на файл — предлагаем reload
  → Панель :changes автоматически обновляется
```

**Авто-обновление панели:**
- fsnotify или polling каждые 1-2 сек
- Панель должна быть "живой" для наблюдения за AI

**Детекция изменений файла (надёжнее чем только mtime):**
```go
type FileState struct {
    Inode  uint64    // защита от replace vs edit
    Size   int64
    Mtime  time.Time
}
// При расхождении — подтверждаем content hash для открытых буферов
```

**Ограничения fsnotify:**
- Linux/inotify: не рекурсивный, лимиты `/proc/sys/fs/inotify/max_user_watches`
- macOS/kqueue: требует FD на каждый файл, плохо масштабируется
- **Решение:** гибрид fsnotify + периодический скан для верификации

---

## Архитектура хранения

```
~/.local/state/qedit/
├── projects/                          # Project Mode (non-git)
│   └── <project-hash>/                # SHA256 от абсолютного пути проекта
│       ├── index.json                 # метаданные проекта
│       └── baseline/                  # snapshots файлов
│           ├── src_main.go            # encoded path
│           └── ...
│
└── files/                             # Single File Mode
    └── <file-path-hash>               # snapshot одиночного файла
```

**Определение stateDir при sudo:**
```go
func getStateDir() string {
    // Уважаем XDG_STATE_HOME если задан
    if xdgState := os.Getenv("XDG_STATE_HOME"); xdgState != "" {
        return filepath.Join(xdgState, "qedit")
    }

    home := os.Getenv("HOME")

    // При sudo: $HOME = /root, ищем home реального пользователя
    if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
        if u, err := user.Lookup(sudoUser); err == nil {
            home = u.HomeDir
        }
    }

    return filepath.Join(home, ".local", "state", "qedit")
}

// ВАЖНО: При создании файлов в state директории под sudo
// нужно выставлять владельца через SUDO_UID/SUDO_GID
func chownToRealUser(path string) error {
    uidStr := os.Getenv("SUDO_UID")
    gidStr := os.Getenv("SUDO_GID")
    if uidStr == "" || gidStr == "" {
        return nil // не под sudo
    }
    uid, _ := strconv.Atoi(uidStr)
    gid, _ := strconv.Atoi(gidStr)
    return os.Chown(path, uid, gid)
}
```

### Метаданные проекта (index.json)
```go
type ProjectIndex struct {
    ProjectPath string                 `json:"project_path"`
    CreatedAt   time.Time              `json:"created_at"`
    Files       map[string]FileInfo    `json:"files"`  // path -> info
}

type FileInfo struct {
    RelPath   string    `json:"rel_path"`
    Mtime     time.Time `json:"mtime"`
    Size      int64     `json:"size"`
    Hash      string    `json:"hash"`      // SHA256
}
```

### Когда сохраняется полный snapshot файла (baseline/)
```
Non-git Project Mode:
  - Манифест (hash/size/mtime) создаётся при СТАРТЕ для всех файлов
  - Полная копия файла сохраняется ЛЕНИВО — только когда нужен restore
  - При restore: если нет копии, читаем оригинал и сохраняем перед восстановлением

Single File Mode:
  - Snapshot сохраняется сразу при открытии файла
```

**Ограничения для манифеста (non-git):**
- Пропускать бинарные файлы (определяем по первым 512 байт)
- Пропускать файлы > 1MB
- Пропускать node_modules, vendor, .git, etc. (через .gitignore если есть)

---

## Новый пакет: `internal/changes`

### manager.go
```go
type ManagerMode int
const (
    ModeProject    ManagerMode = iota  // CWD-based project
    ModeSingleFile                      // Single file outside project (sudo qedit /etc/hosts)
)

// Manager handles local changes detection and restoration
type Manager struct {
    mode            ManagerMode
    projectRoot     string                  // CWD - корень проекта (для ModeProject)
    singleFilePath  string                  // путь к файлу (для ModeSingleFile)
    gitRoot         string                  // git repo root (empty if not in git)
    isGitRepo       bool
    stateDir        string                  // ~/.local/state/qedit/...
    index           *ProjectIndex           // для non-git проектов
    restoreUndo     map[string][]byte       // path -> content before restore (для undo)
}

func NewManager(projectRoot string, filePath string) (*Manager, error)
func (m *Manager) Mode() ManagerMode
func (m *Manager) IsGitRepo() bool

// Получить список изменённых файлов
func (m *Manager) GetChangedFiles() ([]ChangedFile, error)

// Получить diff для конкретного файла
// liveContent — содержимое из памяти (если файл открыт), иначе nil (читаем с диска)
func (m *Manager) GetFileDiff(relPath string, liveContent []byte) ([]DiffHunk, error)

// Восстановить файл к baseline/HEAD (сохраняет текущую версию для undo)
func (m *Manager) RestoreFile(relPath string) error

// Отменить последний restore для файла
func (m *Manager) UndoRestore(relPath string) error

// Проверить, есть ли undo для файла
func (m *Manager) HasUndoRestore(relPath string) bool

// Восстановить все файлы
func (m *Manager) RestoreAll() error

// Создать baseline для файла (вызывается при открытии, если не git)
func (m *Manager) CreateBaseline(path string, content []byte) error

// Проверить внешние изменения в открытых файлах
func (m *Manager) CheckExternalChanges(openFiles []string) []string
```

### types.go
```go
type FileStatus int
const (
    StatusModified FileStatus = iota
    StatusAdded
    StatusDeleted
    StatusUntracked  // новый файл, не в git
    StatusRenamed    // переименован (git detect)
    StatusConflict   // unmerged (merge/rebase conflict)
)

type ChangedFile struct {
    Path        string
    RelPath     string      // относительный путь для отображения
    Status      FileStatus
    HasStaged   bool        // есть изменения в index (staged) — только для Git
    HasUnstaged bool        // есть изменения в worktree (unstaged) — только для Git
}

type DiffHunk struct {
    OldStart int
    OldCount int
    NewStart int
    NewCount int
    Lines    []DiffLine
}

type DiffLine struct {
    Type    LineType  // Context, Added, Removed
    Content string
    OldNum  int       // номер строки в старой версии (0 если Added)
    NewNum  int       // номер строки в новой версии (0 если Removed)
}

type LineType int
const (
    LineContext LineType = iota
    LineAdded
    LineRemoved
)
```

### diff.go
```go
// ComputeDiff вычисляет unified diff между двумя версиями
func ComputeDiff(oldContent, newContent string) []DiffHunk

// Использовать github.com/hexops/gotextdiff или собственную реализацию
```

---

## Интеграция в Editor

### Новые поля в struct Editor
```go
changesManager      *changes.Manager
changesPanelActive  bool              // показана ли панель Local Changes
changedFiles        []changes.ChangedFile
changesFileIndex    int               // выбранный файл в списке
changesScroll       int               // прокрутка diff панели
changesCollapsed    map[string]bool   // свёрнутые файлы
```

### Новый режим
```go
ModeChangesPanel Mode = 5
```

### Хуки
```go
// При открытии файла - создать baseline (если не git)
func (e *Editor) Open(path string) error {
    // ... существующий код ...
    if !e.changesManager.IsGitRepo(path) {
        e.changesManager.CreateBaseline(path, content)
    }
}

// При возврате фокуса - проверить внешние изменения
func (e *Editor) OnFocus() {
    // Проверить mtime, предложить reload если изменился
}
```

---

## Команды

| Команда                | Действие                             |
|------------------------|--------------------------------------|
| `:changes`             | Открыть панель Local Changes         |
| `:changes restore`     | Восстановить текущий файл к baseline |
| `:changes restore all` | Восстановить все файлы               |

---

## Клавиши в режиме ChangesPanel

| Клавиша               | Действие                                                                                |
|-----------------------|-----------------------------------------------------------------------------------------|
| `↑/↓` или `j/k`       | Навигация по файлам (левая панель)                                                      |
| `Shift+↑/↓` или `J/K` | Скролл diff (правая панель)                                                             |
| `PgUp/PgDn`           | Страница вверх/вниз (в активной панели)                                                 |
| `Home/End`            | В начало/конец списка или diff                                                          |
| `Tab`                 | Переключение фокуса: список ↔ diff                                                      |
| `Enter`               | Открыть выбранный файл в редакторе                                                      |
| `r`                   | **Restore file** — discard unstaged (Git) / restore из baseline (non-git)               |
| `Shift+R`             | **Discard ALL for file** — staged+unstaged из HEAD **(подтверждение если есть staged)** |
| `u`                   | Undo restore — отменить последний откат                                                 |
| `Ctrl+Shift+R`        | **Restore ALL files** **(с подтверждением!)**                                           |
| `Esc` / `q`           | Закрыть панель                                                                          |

### Undo Restore (защита от случайного отката)

```
Пользователь нажимает 'r':
  1. Сохраняем текущую версию файла в restoreUndo[path]
  2. Восстанавливаем файл к baseline/HEAD
  3. Показываем toast: "Restored. Press 'u' to undo"
  4. Если ошибся — 'u' возвращает версию из restoreUndo
```

**Restore All (`Ctrl+Shift+R`) — с обязательным подтверждением:**
```
Пользователь нажимает 'Ctrl+Shift+R':
  → Показываем prompt: "Restore all N files? [y/N]"
  → Только при 'y' выполняем массовый откат
```

**Ограничения:**
- Буфер хранится только в памяти (до закрытия панели)
- Один уровень undo на файл (повторный restore перезаписывает буфер)

---

## Файлы для изменений

| Файл                               | Изменения                                                       |
|------------------------------------|-----------------------------------------------------------------|
| `internal/changes/manager.go`      | **NEW** - менеджер изменений (ModeProject/ModeSingleFile)       |
| `internal/changes/types.go`        | **NEW** - структуры данных (ChangedFile, HasStaged/HasUnstaged) |
| `internal/changes/git.go`          | **NEW** - git integration (porcelain v2, restore семантика)     |
| `internal/changes/baseline.go`     | **NEW** - Project Baseline + single file mode                   |
| `internal/changes/state.go`        | **NEW** - getStateDir(), chownToRealUser()                      |
| `internal/changes/diff.go`         | **NEW** - вычисление diff                                       |
| `internal/changes/ignore.go`       | **NEW** - парсинг .gitignore для non-git проектов               |
| `internal/editor/editor.go`        | Добавить поля, режим, инициализация Manager                     |
| `internal/editor/changes_panel.go` | **NEW** - UI панели (включая undo restore)                      |
| `internal/app/app.go`              | Передать CWD и filePath в Editor                                |
| `go.mod`                           | Добавить: `github.com/hexops/gotextdiff`                        |

---

## Порядок реализации

### Фаза 1: Core - Git Integration
1. Создать `internal/changes/` пакет
2. Реализовать `git.go`:
   - `IsGitRepo()` - определить git репозиторий
   - `GetGitChangedFiles()` - `git status --porcelain=v2 -z` (v2 формат + NUL-разделители)
   - `GetGitBaseContent(path)` - `git show HEAD:<path>` (содержимое из HEAD)
   - **Restore (современный подход через `git restore`):**
     - `RestoreUnstaged(path)` - `git restore <path>` (worktree из index)
     - `RestoreAll(path)` - `git restore --staged --worktree <path>` (всё из HEAD)
     - `RestoreUntracked(path)` - удалить файл (с сохранением в undo)
   - Парсить staged/unstaged флаги из porcelain v2 output
3. Написать тесты

### Фаза 2: Core - Baseline (non-git + single file)
1. Реализовать `state.go`:
   - `getStateDir()` - XDG_STATE_HOME + $SUDO_USER
   - `chownToRealUser()` - правильный владелец при sudo
2. Реализовать `baseline.go`:
   - `CreateProjectManifest()` - индексация проекта при старте
   - `ScanForChanges()` - быстрый скан size+mtime, потом hash
   - `GetFileBaseline()` / `SaveFileBaseline()` - lazy копии
   - `LoadProjectIndex()` / `SaveProjectIndex()`
3. Реализовать `diff.go` - вычисление diff между файлами
4. Реализовать `ignore.go` - парсинг .gitignore для исключений
5. Написать тесты

### Фаза 3: Manager
1. Реализовать `manager.go`:
   - Два режима: ModeProject / ModeSingleFile
   - Определение режима при создании
   - `RestoreFile()` с сохранением в restoreUndo
   - `UndoRestore()` - отмена последнего отката
2. `GetChangedFiles()`, `GetFileDiff()`
3. Написать тесты

### Фаза 4: UI - Changes Panel
1. Добавить `ModeChangesPanel` в Editor
2. Реализовать `renderChangesPanel()` - split view как в Zed
3. Реализовать `handleChangesPanel()`:
   - `r` - restore (с сохранением для undo)
   - `u` - undo restore
   - Остальные клавиши
4. Интегрировать с Editor lifecycle

### Фаза 5: Polish
1. Подсветка +/- строк (зелёный/красный)
2. Collapse/expand для файлов
3. Обработка внешних изменений в открытых буферах
4. Статус в панели: "Undo available" когда есть что отменить

---

## Верификация

```bash
make build && make lint && make test
```

### Тест 1: Изменения в редакторе (git репо)
1. Открыть файл, сделать изменения
2. `:changes` - должен показать панель с изменёнными файлами
3. Видеть inline diff справа
4. `r` - restore файла к HEAD

### Тест 2: Изменения от AI агента (git репо)
1. Запустить редактор в git репо
2. **Не трогая редактор**, изменить файл извне (симуляция AI агента):
   ```bash
   echo "// AI change" >> somefile.go
   ```
3. `:changes` - должен показать изменённый файл!
4. `r` - restore к HEAD

### Тест 3: Non-git проект
1. Создать папку без git, добавить файлы
2. Запустить редактор из этой папки
3. Изменить файл извне
4. `:changes` - показать diff vs baseline
5. `r` - restore к baseline

### Тест 4: Reload при внешних изменениях
1. Открыть файл в редакторе
2. Изменить этот файл извне
3. Вернуться в редактор
4. Должен появиться prompt "File changed externally. Reload?"

### Тест 5: Undo Restore
1. Запустить редактор в git репо
2. Изменить файл, открыть `:changes`
3. Нажать `r` — файл откатился
4. Нажать `u` — изменения вернулись
5. Файл снова в изменённом состоянии

### Тест 6: Single File Mode (sudo)
1. `sudo qedit /etc/hosts`
2. Добавить строку
3. `:changes` — видим только /etc/hosts
4. Diff показывает добавленную строку
5. `r` — откатывает к исходному
6. Snapshot сохранён в `~/.local/state/qedit/files/` (не /root)

### Тест 7: Project Baseline (non-git) — AI изменения
1. Создать папку без git с 10 файлами
2. Запустить редактор (создаётся манифест)
3. **Не открывая** utils.go, изменить его извне (симуляция AI)
4. `:changes` — должен показать utils.go как изменённый!
5. `r` — restore работает даже для неоткрытого файла

### Тест 8: Staged vs Unstaged (git)
1. Изменить файл, застейджить часть: `git add -p`
2. Сделать ещё изменения (unstaged)
3. `:changes` — файл показывает `[S][M]`
4. `r` — сбрасывает только unstaged (staged сохраняются!)
5. `Shift+R` — сбрасывает всё (с подтверждением)
