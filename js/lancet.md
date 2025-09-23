# Lancet Go Library - Подробная документация функций

Lancet - это всеобъемлющая, эффективная и переиспользуемая библиотека утилитарных функций для Go. Вдохновлена Java Apache Common Package и Lodash.js.

## Основная информация

- **700+ функций Go** поддерживающих строки, слайсы, дату/время, сеть, криптографию и многое другое
- **Зависимости:** только стандартная библиотека Go и golang.org/x
- **Поддержка Go 1.18+** с использованием дженериков (v2.x.x)
- **Полное покрытие тестами** для каждой экспортируемой функции

## Установка

```bash
# Go 1.18+
go get github.com/duke-git/lancet/v2

# Go < 1.18
go get github.com/duke-git/lancet
```

---

## 1. АЛГОРИТМЫ (Algorithm)

### Пакет: `github.com/duke-git/lancet/v2/algorithm`

| Функция | Описание | Параметры | Возвращает | Сложность |
|---------|----------|-----------|------------|-----------|
| **BubbleSort** | Сортировка пузырьком, изменяет исходный слайс | `slice []T` | `[]T` | O(n²) |
| **CountSort** | Сортировка подсчетом, не изменяет исходный слайс | `slice []T` | `[]T` | O(n+k) |
| **HeapSort** | Пирамидальная сортировка, изменяет исходный слайс | `slice []T` | `[]T` | O(n log n) |
| **InsertionSort** | Сортировка вставками, изменяет исходный слайс | `slice []T` | `[]T` | O(n²) |
| **MergeSort** | Сортировка слиянием, изменяет исходный слайс | `slice []T` | `[]T` | O(n log n) |
| **QuickSort** | Быстрая сортировка, изменяет исходный слайс | `slice []T` | `[]T` | O(n log n) |
| **SelectionSort** | Сортировка выбором, изменяет исходный слайс | `slice []T` | `[]T` | O(n²) |
| **ShellSort** | Сортировка Шелла, изменяет исходный слайс | `slice []T` | `[]T` | O(n^1.5) |
| **BinarySearch** | Бинарный поиск (рекурсивный) | `slice []T, target T` | `int` | O(log n) |
| **BinaryIterativeSearch** | Бинарный поиск (итеративный) | `slice []T, target T` | `int` | O(log n) |
| **LinearSearch** | Линейный поиск | `slice []T, target T, equal func` | `int` | O(n) |
| **LRUCache** | LRU кэш в памяти | `capacity int` | `*LRUCache` | O(1) |

---

## 2. РАБОТА С МАССИВАМИ/СЛАЙСАМИ (Slice)

### Пакет: `github.com/duke-git/lancet/v2/slice`

| Функция | Описание | Параметры | Возвращает | Примечания |
|---------|----------|-----------|------------|------------|
| **AppendIfAbsent** | Добавляет элемент если его нет | `slice []T, item T` | `[]T` | Проверка на существование |
| **Contain** | Проверяет наличие элемента | `slice []T, target T` | `bool` | - |
| **ContainBy** | Проверка через предикат | `slice []T, predicate func` | `bool` | - |
| **ContainSubSlice** | Проверяет наличие подслайса | `slice []T, subSlice []T` | `bool` | - |
| **Chunk** | Разбивает на группы заданного размера | `slice []T, size int` | `[][]T` | - |
| **Compact** | Удаляет falsy значения | `slice []T` | `[]T` | false, nil, 0, "" |
| **Concat** | Объединяет слайсы | `slice []T, others ...[]T` | `[]T` | - |
| **Count** | Подсчитывает вхождения | `slice []T, item T` | `int` | - |
| **CountBy** | Подсчет по предикату | `slice []T, predicate func` | `int` | - |
| **Difference** | Элементы из первого, отсутствующие во втором | `slice1 []T, slice2 []T` | `[]T` | - |
| **DifferenceBy** | Разность по iteratee функции | `slice1 []T, slice2 []T, iteratee func` | `[]T` | - |
| **DifferenceWith** | Разность с компаратором | `slice1 []T, slice2 []T, comparator func` | `[]T` | - |
| **DeleteAt** | Удаляет элемент по индексу | `slice []T, index int` | `[]T` | - |
| **DeleteRange** | Удаляет диапазон элементов | `slice []T, start, end int` | `[]T` | [start, end) |
| **Drop** | Удаляет n элементов с начала | `slice []T, n int` | `[]T` | - |
| **DropRight** | Удаляет n элементов с конца | `slice []T, n int` | `[]T` | - |
| **DropWhile** | Удаляет элементы пока условие true | `slice []T, predicate func` | `[]T` | - |
| **DropRightWhile** | Удаляет с конца пока условие true | `slice []T, predicate func` | `[]T` | - |
| **Equal** | Проверяет равенство слайсов | `slice1 []T, slice2 []T` | `bool` | Порядок важен |
| **EqualWith** | Равенство с компаратором | `slice1 []T, slice2 []T, comparator func` | `bool` | - |
| **EqualUnordered** | Равенство без учета порядка | `slice1 []T, slice2 []T` | `bool` | - |
| **Every** | Все элементы проходят тест | `slice []T, predicate func` | `bool` | - |
| **Filter** | Фильтрует элементы | `slice []T, predicate func` | `[]T` | - |
| **FilterMap** | Фильтр + мапинг одновременно | `slice []T, iteratee func` | `[]U` | - |
| **FindBy** | Находит первый элемент по условию | `slice []T, predicate func` | `T, bool` | - |
| **FindLastBy** | Находит последний элемент по условию | `slice []T, predicate func` | `T, bool` | - |
| **Flatten** | Выравнивает на один уровень | `slice [][]T` | `[]T` | - |
| **FlattenDeep** | Рекурсивное выравнивание | `slice interface{}` | `[]interface{}` | - |
| **FlatMap** | Мапинг + выравнивание | `slice []T, iteratee func` | `[]U` | - |
| **ForEach** | Итерация по элементам | `slice []T, iteratee func` | `void` | - |
| **ForEachWithBreak** | Итерация с возможностью прерывания | `slice []T, iteratee func` | `void` | - |
| **ForEachConcurrent** | Параллельная итерация | `slice []T, iteratee func` | `void` | - |
| **GroupBy** | Группирует элементы | `slice []T, iteratee func` | `[][]T` | - |
| **GroupWith** | Группировка в map | `slice []T, iteratee func` | `map[K][]T` | - |
| **Intersection** | Пересечение слайсов | `slices ...[]T` | `[]T` | - |
| **InsertAt** | Вставляет по индексу | `slice []T, index int, values ...T` | `[]T` | - |
| **IndexOf** | Индекс первого вхождения | `slice []T, item T` | `int` | -1 если не найден |
| **LastIndexOf** | Индекс последнего вхождения | `slice []T, item T` | `int` | -1 если не найден |
| **Map** | Преобразует элементы | `slice []T, iteratee func` | `[]U` | - |
| **MapConcurrent** | Параллельное преобразование | `slice []T, iteratee func` | `[]U` | - |
| **Merge** | Объединяет слайсы | `slices ...[]T` | `[]T` | - |
| **Reverse** | Разворачивает порядок | `slice []T` | `[]T` | - |
| **ReduceBy** | Сворачивает к одному значению | `slice []T, iteratee func, initial U` | `U` | - |
| **ReduceRight** | Свертка справа налево | `slice []T, iteratee func, initial U` | `U` | - |
| **ReduceConcurrent** | Параллельная свертка | `slice []T, iteratee func, initial U` | `U` | - |
| **Replace** | Заменяет первые n вхождений | `slice []T, old T, new T, n int` | `[]T` | - |
| **ReplaceAll** | Заменяет все вхождения | `slice []T, old T, new T` | `[]T` | - |
| **Repeat** | Создает слайс с повторением | `item T, n int` | `[]T` | - |
| **Shuffle** | Перемешивает элементы | `slice []T` | `[]T` | Изменяет исходный |
| **ShuffleCopy** | Перемешивает копию | `slice []T` | `[]T` | Новый слайс |
| **Sort** | Сортирует слайс | `slice []T` | `[]T` | Для упорядоченных типов |
| **SortBy** | Сортировка с функцией сравнения | `slice []T, less func` | `[]T` | - |
| **Some** | Есть ли элементы, проходящие тест | `slice []T, predicate func` | `bool` | - |
| **SymmetricDifference** | Симметричная разность | `slice1 []T, slice2 []T` | `[]T` | - |
| **ToSlice** | Преобразует аргументы в слайс | `values ...T` | `[]T` | - |
| **ToSlicePointer** | Преобразует в указатель на слайс | `values ...T` | `*[]T` | - |
| **Unique** | Удаляет дубликаты | `slice []T` | `[]T` | - |
| **UniqueBy** | Удаляет дубликаты по функции | `slice []T, iteratee func` | `[]T` | - |
| **Union** | Объединение с уникальными элементами | `slices ...[]T` | `[]T` | - |
| **UnionBy** | Объединение по iteratee | `slice1 []T, slice2 []T, iteratee func` | `[]T` | - |
| **UpdateAt** | Обновляет элемент по индексу | `slice []T, index int, value T` | `[]T` | - |
| **Without** | Исключает заданные элементы | `slice []T, values ...T` | `[]T` | - |
| **KeyBy** | Преобразует в map по ключу | `slice []T, iteratee func` | `map[K]T` | - |
| **Join** | Объединяет в строку | `slice []T, separator string` | `string` | - |
| **Partition** | Разделяет по условию | `slice []T, predicate func` | `[]T, []T` | true, false |

---

## 3. РАБОТА СО СТРОКАМИ (Strutil)

### Пакет: `github.com/duke-git/lancet/v2/strutil`

| Функция | Описание | Параметры | Возвращает | Примеры |
|---------|----------|-----------|------------|---------|
| **After** | Подстрока после первого вхождения | `str, substr string` | `string` | After("hello world", " ") → "world" |
| **AfterLast** | Подстрока после последнего вхождения | `str, substr string` | `string` | AfterLast("a.b.c", ".") → "c" |
| **Before** | Подстрока до первого вхождения | `str, substr string` | `string` | Before("hello world", " ") → "hello" |
| **BeforeLast** | Подстрока до последнего вхождения | `str, substr string` | `string` | BeforeLast("a.b.c", ".") → "a.b" |
| **CamelCase** | Преобразует в camelCase | `str string` | `string` | CamelCase("hello_world") → "helloWorld" |
| **Capitalize** | Первая буква заглавная, остальные строчные | `str string` | `string` | Capitalize("hello") → "Hello" |
| **ContainsAll** | Содержит ли все подстроки | `str string, substrs []string` | `bool` | - |
| **ContainsAny** | Содержит ли любую из подстрок | `str string, substrs []string` | `bool` | - |
| **IsString** | Проверяет, является ли значение строкой | `v interface{}` | `bool` | - |
| **KebabCase** | Преобразует в kebab-case | `str string` | `string` | KebabCase("HelloWorld") → "hello-world" |
| **UpperKebabCase** | Преобразует в UPPER-KEBAB-CASE | `str string` | `string` | UpperKebabCase("hello") → "HELLO" |
| **LowerFirst** | Первая буква строчная | `str string` | `string` | LowerFirst("Hello") → "hello" |
| **UpperFirst** | Первая буква заглавная | `str string` | `string` | UpperFirst("hello") → "Hello" |
| **Pad** | Дополняет строку с обеих сторон | `str string, size int, padStr string` | `string` | - |
| **PadEnd** | Дополняет справа | `str string, size int, padStr string` | `string` | - |
| **PadStart** | Дополняет слева | `str string, size int, padStr string` | `string` | - |
| **Reverse** | Разворачивает строку | `str string` | `string` | Reverse("hello") → "olleh" |
| **SnakeCase** | Преобразует в snake_case | `str string` | `string` | SnakeCase("HelloWorld") → "hello_world" |
| **UpperSnakeCase** | Преобразует в UPPER_SNAKE_CASE | `str string` | `string` | UpperSnakeCase("hello") → "HELLO" |
| **SplitEx** | Разделяет с контролем пустых строк | `str, separator string, removeEmpty bool` | `[]string` | - |
| **Substring** | Извлекает подстроку | `str string, start, length int` | `string` | - |
| **Wrap** | Обертывает строку | `str, wrapper string` | `string` | Wrap("hello", "*") → "*hello*" |
| **Unwrap** | Разворачивает строку | `str, wrapper string` | `string` | Unwrap("*hello*", "*") → "hello" |
| **SplitWords** | Разделяет на слова | `str string` | `[]string` | Только буквенные символы |
| **WordCount** | Считает слова | `str string` | `int` | - |
| **RemoveNonPrintable** | Удаляет непечатаемые символы | `str string` | `string` | - |
| **StringToBytes** | Преобразует в []byte без копирования | `str string` | `[]byte` | Небезопасно |
| **BytesToString** | Преобразует из []byte без копирования | `bytes []byte` | `string` | Небезопасно |
| **IsBlank** | Проверяет на пустоту/пробелы | `str string` | `bool` | - |
| **IsNotBlank** | Проверяет на не пустоту | `str string` | `bool` | - |
| **HasPrefixAny** | Начинается с любого из префиксов | `str string, prefixes []string` | `bool` | - |
| **HasSuffixAny** | Заканчивается любым из суффиксов | `str string, suffixes []string` | `bool` | - |
| **IndexOffset** | Индекс после смещения | `str, substr string, offset int` | `int` | - |
| **ReplaceWithMap** | Замена по карте | `str string, replaceMap map[string]string` | `string` | - |
| **Trim** | Обрезает символы | `str, cutset string` | `string` | - |
| **SplitAndTrim** | Разделяет и обрезает | `str, delimiter, cutset string` | `[]string` | - |
| **HideString** | Скрывает символы | `str string, start, end int, replaceChar rune` | `string` | - |
| **RemoveWhiteSpace** | Удаляет пробельные символы | `str string` | `string` | - |
| **SubInBetween** | Подстрока между позициями | `str string, start, end int` | `string` | - |
| **HammingDistance** | Расстояние Хэмминга | `str1, str2 string` | `int` | - |
| **Concat** | Объединяет строки | `strs ...string` | `string` | - |
| **Ellipsis** | Обрезает с многоточием | `str string, length int` | `string` | - |

---

## 4. МАТЕМАТИЧЕСКИЕ ФУНКЦИИ (Mathutil)

### Пакет: `github.com/duke-git/lancet/v2/mathutil`

| Функция | Описание | Параметры | Возвращает | Ограничения |
|---------|----------|-----------|------------|-------------|
| **Average** | Среднее значение | `numbers ...T` | `T` | T: числовой тип |
| **Exponent** | Возведение в степень | `x, n int64` | `int64` | int64 только |
| **Fibonacci** | Число Фибоначчи | `n int` | `int` | n >= 0 |
| **Factorial** | Факториал | `x uint` | `uint` | uint только |
| **Max** | Максимальное значение | `numbers ...T` | `T` | T: сравнимый |
| **MaxBy** | Максимум с компаратором | `slice []T, comparator func` | `T` | - |
| **Min** | Минимальное значение | `numbers ...T` | `T` | T: сравнимый |
| **MinBy** | Минимум с компаратором | `slice []T, comparator func` | `T` | - |
| **Percent** | Процент от общего | `val, total int` | `float64` | total > 0 |
| **RoundToFloat** | Округление до n знаков | `x float64, n int` | `float64` | - |
| **RoundToString** | Округление до строки | `x float64, n int` | `string` | - |
| **TruncRound** | Округление int64 | `x float64, n int` | `int64` | - |
| **CeilToFloat** | Округление вверх | `x float64, n int` | `float64` | - |
| **CeilToString** | Округление вверх до строки | `x float64, n int` | `string` | - |
| **FloorToFloat** | Округление вниз | `x float64, n int` | `float64` | - |
| **FloorToString** | Округление вниз до строки | `x float64, n int` | `string` | - |
| **Range** | Диапазон чисел | `start, count int` | `[]int` | шаг = 1 |
| **RangeWithStep** | Диапазон с шагом | `start, end, step int` | `[]int` | - |
| **AngleToRadian** | Градусы в радианы | `angle float64` | `float64` | - |
| **RadianToAngle** | Радианы в градусы | `radian float64` | `float64` | - |
| **PointDistance** | Расстояние между точками | `x1,y1,x2,y2 float64` | `float64` | - |
| **IsPrime** | Проверка на простоту | `n int` | `bool` | n > 1 |
| **GCD** | НОД чисел | `integers ...int` | `int` | - |
| **LCM** | НОК чисел | `integers ...int` | `int` | - |
| **Cos** | Косинус | `radian float64` | `float64` | - |
| **Sin** | Синус | `radian float64` | `float64` | - |
| **Log** | Логарифм по основанию | `n, base float64` | `float64` | n,base > 0 |
| **Sum** | Сумма чисел | `numbers ...T` | `T` | T: числовой |
| **Abs** | Абсолютное значение | `x T` | `T` | T: числовой |
| **Div** | Деление | `x, y T` | `T` | y != 0 |
| **Variance** | Дисперсия | `numbers ...float64` | `float64` | - |
| **StdDev** | Стандартное отклонение | `numbers ...float64` | `float64` | - |
| **Permutation** | Размещения P(n,k) | `n, k int64` | `int64` | n >= k >= 0 |
| **Combination** | Сочетания C(n,k) | `n, k int64` | `int64` | n >= k >= 0 |

---

## 5. РАБОТА С ДАТОЙ И ВРЕМЕНЕМ (Datetime)

### Пакет: `github.com/duke-git/lancet/v2/datetime`

| Функция | Описание | Параметры | Возвращает | Примеры |
|---------|----------|-----------|------------|---------|
| **AddDay** | Добавляет/убавляет дни | `t time.Time, day int` | `time.Time` | - |
| **AddHour** | Добавляет/убавляет часы | `t time.Time, hour int` | `time.Time` | - |
| **AddMinute** | Добавляет/убавляет минуты | `t time.Time, minute int` | `time.Time` | - |
| **AddWeek** | Добавляет/убавляет недели | `t time.Time, week int` | `time.Time` | - |
| **AddMonth** | Добавляет/убавляет месяцы | `t time.Time, month int` | `time.Time` | - |
| **AddYear** | Добавляет/убавляет годы | `t time.Time, year int` | `time.Time` | - |
| **AddDaySafe** | Безопасное добавление дней | `t time.Time, day int` | `time.Time` | Проверяет границы месяца |
| **AddMonthSafe** | Безопасное добавление месяцев | `t time.Time, month int` | `time.Time` | Проверяет границы месяца |
| **AddYearSafe** | Безопасное добавление лет | `t time.Time, year int` | `time.Time` | Проверяет границы месяца |
| **BeginOfMinute** | Начало минуты | `t time.Time` | `time.Time` | 00 секунд |
| **BeginOfHour** | Начало часа | `t time.Time` | `time.Time` | 00:00 |
| **BeginOfDay** | Начало дня | `t time.Time` | `time.Time` | 00:00:00 |
| **BeginOfWeek** | Начало недели | `t time.Time` | `time.Time` | Понедельник 00:00:00 |
| **BeginOfMonth** | Начало месяца | `t time.Time` | `time.Time` | 1 число 00:00:00 |
| **BeginOfYear** | Начало года | `t time.Time` | `time.Time` | 1 января 00:00:00 |
| **EndOfMinute** | Конец минуты | `t time.Time` | `time.Time` | 59 секунд |
| **EndOfHour** | Конец часа | `t time.Time` | `time.Time` | 59:59 |
| **EndOfDay** | Конец дня | `t time.Time` | `time.Time` | 23:59:59 |
| **EndOfWeek** | Конец недели | `t time.Time` | `time.Time` | Воскресенье 23:59:59 |
| **EndOfMonth** | Конец месяца | `t time.Time` | `time.Time` | Последний день 23:59:59 |
| **EndOfYear** | Конец года | `t time.Time` | `time.Time` | 31 декабря 23:59:59 |
| **GetNowDate** | Текущая дата | `()` | `string` | "2023-01-01" |
| **GetNowTime** | Текущее время | `()` | `string` | "15:04:05" |
| **GetNowDateTime** | Текущие дата и время | `()` | `string` | "2023-01-01 15:04:05" |
| **GetTodayStartTime** | Начало сегодняшнего дня | `()` | `string` | "2023-01-01 00:00:00" |
| **GetTodayEndTime** | Конец сегодняшнего дня | `()` | `string` | "2023-01-01 23:59:59" |
| **GetZeroHourTimestamp** | Timestamp начала дня | `t time.Time` | `int64` | - |
| **GetNightTimestamp** | Timestamp конца дня | `t time.Time` | `int64` | - |
| **FormatTimeToStr** | Время в строку | `t time.Time, format string` | `string` | - |
| **FormatStrToTime** | Строку во время | `str, format string` | `time.Time, error` | - |
| **NewUnix** | Создать время из Unix timestamp | `timestamp int64` | `time.Time` | - |
| **NewUnixNow** | Текущий Unix timestamp | `()` | `int64` | - |
| **NewFormat** | Время из строки стандартного формата | `str string` | `time.Time, error` | "2006-01-02 15:04:05" |
| **NewISO8601** | Время из ISO8601 | `str string` | `time.Time, error` | - |
| **ToUnix** | Время в Unix timestamp | `t time.Time` | `int64` | - |
| **ToFormat** | Время в стандартный формат | `t time.Time` | `string` | "2006-01-02 15:04:05" |
| **ToFormatForTpl** | Время в заданный формат | `t time.Time, template string` | `string` | - |
| **ToIso8601** | Время в ISO8601 | `t time.Time` | `string` | - |
| **IsLeapYear** | Проверка високосного года | `year int` | `bool` | - |
| **BetweenSeconds** | Разность в секундах | `t1, t2 time.Time` | `int64` | - |
| **DayOfYear** | День года | `t time.Time` | `int` | 1-366 |
| **IsWeekend** | Проверка выходного | `t time.Time` | `bool` | Суббота/воскресенье |
| **NowDateOrTime** | Текущее время с форматом | `format, timezone string` | `string` | - |
| **Timestamp** | Timestamp в секундах | `()` | `int64` | - |
| **TimestampMilli** | Timestamp в миллисекундах | `()` | `int64` | - |
| **TimestampMicro** | Timestamp в микросекундах | `()` | `int64` | - |
| **TimestampNano** | Timestamp в наносекундах | `()` | `int64` | - |
| **TrackFuncTime** | Измерение времени выполнения функции | `fn func()` | `time.Duration` | - |
| **DaysBetween** | Дни между датами | `t1, t2 time.Time` | `int` | - |

---

## 6. КРИПТОГРАФИЯ И ШИФРОВАНИЕ (Cryptor)

### Пакет: `github.com/duke-git/lancet/v2/cryptor`

| Функция | Описание | Параметры | Возвращает | Алгоритм |
|---------|----------|-----------|------------|----------|
| **AesEcbEncrypt** | AES ECB шифрование | `data, key []byte` | `[]byte, error` | AES-ECB |
| **AesEcbDecrypt** | AES ECB дешифрование | `data, key []byte` | `[]byte, error` | AES-ECB |
| **AesCbcEncrypt** | AES CBC шифрование | `data, key []byte` | `[]byte, error` | AES-CBC |
| **AesCbcDecrypt** | AES CBC дешифрование | `data, key []byte` | `[]byte, error` | AES-CBC |
| **AesCtrCrypt** | AES CTR шифрование/дешифрование | `data, key []byte` | `[]byte, error` | AES-CTR |
| **AesCfbEncrypt** | AES CFB шифрование | `data, key []byte` | `[]byte, error` | AES-CFB |
| **AesCfbDecrypt** | AES CFB дешифрование | `data, key []byte` | `[]byte, error` | AES-CFB |
| **AesOfbEncrypt** | AES OFB шифрование | `data, key []byte` | `[]byte, error` | AES-OFB |
| **AesOfbDecrypt** | AES OFB дешифрование | `data, key []byte` | `[]byte, error` | AES-OFB |
| **AesGcmEncrypt** | AES GCM шифрование | `data, key []byte` | `[]byte, error` | AES-GCM |
| **AesGcmDecrypt** | AES GCM дешифрование | `data, key []byte` | `[]byte, error` | AES-GCM |
| **Base64StdEncode** | Base64 кодирование | `data []byte` | `string` | Base64 Standard |
| **Base64StdDecode** | Base64 декодирование | `data string` | `[]byte, error` | Base64 Standard |
| **DesEcbEncrypt** | DES ECB шифрование | `data, key []byte` | `[]byte, error` | DES-ECB |
| **DesEcbDecrypt** | DES ECB дешифрование | `data, key []byte` | `[]byte, error` | DES-ECB |
| **DesCbcEncrypt** | DES CBC шифрование | `data, key []byte` | `[]byte, error` | DES-CBC |
| **DesCbcDecrypt** | DES CBC дешифрование | `data, key []byte` | `[]byte, error` | DES-CBC |
| **DesCtrCrypt** | DES CTR шифрование/дешифрование | `data, key []byte` | `[]byte, error` | DES-CTR |
| **DesCfbEncrypt** | DES CFB шифрование | `data, key []byte` | `[]byte, error` | DES-CFB |
| **DesCfbDecrypt** | DES CFB дешифрование | `data, key []byte` | `[]byte, error` | DES-CFB |
| **DesOfbEncrypt** | DES OFB шифрование | `data, key []byte` | `[]byte, error` | DES-OFB |
| **DesOfbDecrypt** | DES OFB дешифрование | `data, key []byte` | `[]byte, error` | DES-OFB |
| **HmacMd5** | HMAC-MD5 хэш | `data, key string` | `string` | HMAC-MD5 |
| **HmacMd5WithBase64** | HMAC-MD5 с Base64 | `data, key string` | `string` | HMAC-MD5 + Base64 |
| **HmacSha1** | HMAC-SHA1 хэш | `data, key string` | `string` | HMAC-SHA1 |
| **HmacSha1WithBase64** | HMAC-SHA1 с Base64 | `data, key string` | `string` | HMAC-SHA1 + Base64 |
| **HmacSha256** | HMAC-SHA256 хэш | `data, key string` | `string` | HMAC-SHA256 |
| **HmacSha256WithBase64** | HMAC-SHA256 с Base64 | `data, key string` | `string` | HMAC-SHA256 + Base64 |
| **HmacSha512** | HMAC-SHA512 хэш | `data, key string` | `string` | HMAC-SHA512 |
| **HmacSha512WithBase64** | HMAC-SHA512 с Base64 | `data, key string` | `string` | HMAC-SHA512 + Base64 |
| **Md5Byte** | MD5 хэш байтов | `data []byte` | `string` | MD5 |
| **Md5ByteWithBase64** | MD5 хэш байтов с Base64 | `data []byte` | `string` | MD5 + Base64 |
| **Md5String** | MD5 хэш строки | `data string` | `string` | MD5 |
| **Md5StringWithBase64** | MD5 хэш строки с Base64 | `data string` | `string` | MD5 + Base64 |
| **Md5File** | MD5 хэш файла | `filepath string` | `string, error` | MD5 |
| **Sha1** | SHA1 хэш | `data string` | `string` | SHA1 |
| **Sha1WithBase64** | SHA1 хэш с Base64 | `data string` | `string` | SHA1 + Base64 |
| **Sha256** | SHA256 хэш | `data string` | `string` | SHA256 |
| **Sha256WithBase64** | SHA256 хэш с Base64 | `data string` | `string` | SHA256 + Base64 |
| **Sha512** | SHA512 хэш | `data string` | `string` | SHA512 |
| **Sha512WithBase64** | SHA512 хэш с Base64 | `data string` | `string` | SHA512 + Base64 |
| **GenerateRsaKey** | Генерация RSA ключей | `keySize int, priKeyFile, pubKeyFile string` | `error` | RSA |
| **RsaEncrypt** | RSA шифрование | `data []byte, pubKey string` | `[]byte, error` | RSA |
| **RsaDecrypt** | RSA дешифрование | `data []byte, priKey string` | `[]byte, error` | RSA |
| **GenerateRsaKeyPair** | Генерация пары RSA ключей | `keySize int` | `(string, string, error)` | RSA |
| **RsaEncryptOAEP** | RSA OAEP шифрование | `data []byte, pubKey string` | `[]byte, error` | RSA-OAEP |
| **RsaDecryptOAEP** | RSA OAEP дешифрование | `data []byte, priKey string` | `[]byte, error` | RSA-OAEP |
| **RsaSign** | RSA подпись | `data []byte, priKey string` | `[]byte, error` | RSA |
| **RsaVerifySign** | Проверка RSA подписи | `data, signature []byte, pubKey string` | `bool` | RSA |

---

## 7. РАБОТА С ФАЙЛАМИ (Fileutil)

### Пакет: `github.com/duke-git/lancet/v2/fileutil`

| Функция | Описание | Параметры | Возвращает | Особенности |
|---------|----------|-----------|------------|-------------|
| **ClearFile** | Очищает файл | `filepath string` | `error` | Записывает пустую строку |
| **CreateFile** | Создает файл | `filepath string` | `bool` | Создает директории если нужно |
| **CreateDir** | Создает директорию | `absPath string` | `error` | Рекурсивно |
| **CopyFile** | Копирует файл | `srcPath, dstPath string` | `error` | - |
| **CopyDir** | Копирует директорию | `srcPath, dstPath string` | `error` | Рекурсивно |
| **FileMode** | Возвращает режим файла | `filepath string` | `os.FileMode, error` | - |
| **MiMeType** | Определяет MIME тип | `filepath string` | `string` | - |
| **IsExist** | Проверяет существование | `path string` | `bool` | Файл или директория |
| **IsLink** | Проверяет символическую ссылку | `filepath string` | `bool` | - |
| **IsDir** | Проверяет директорию | `path string` | `bool` | - |
| **ListFileNames** | Список имен файлов | `path string` | `[]string, error` | В директории |
| **RemoveFile** | Удаляет файл | `filepath string` | `error` | - |
| **RemoveDir** | Удаляет директорию | `absPath string` | `error` | Рекурсивно |
| **ReadFileToString** | Читает файл в строку | `filepath string` | `string, error` | - |
| **ReadFileByLine** | Читает файл по строкам | `filepath string` | `[]string, error` | - |
| **Zip** | Создает ZIP архив | `fpath, destPath string` | `error` | Файл или директория |
| **ZipAppendEntry** | Добавляет в ZIP | `fpath, destPath string` | `error` | К существующему архиву |
| **UnZip** | Распаковывает ZIP | `zipPath, destPath string` | `error` | - |
| **CurrentPath** | Текущий путь | `()` | `string, error` | Абсолютный путь |
| **IsZipFile** | Проверяет ZIP файл | `filepath string` | `bool` | - |
| **FileSize** | Размер файла | `filepath string` | `int64, error` | В байтах |
| **MTime** | Время модификации | `filepath string` | `int64, error` | Unix timestamp |
| **Sha** | SHA хэш файла | `filepath string, shaType ...int` | `string, error` | SHA1/256/512 |
| **ReadCsvFile** | Читает CSV файл | `filepath string` | `[][]string, error` | - |
| **WriteCsvFile** | Записывает CSV файл | `filepath string, data [][]string` | `error` | - |
| **WriteMapsToCsv** | Записывает maps в CSV | `filepath string, data []map[string]string` | `error` | - |
| **WriteBytesToFile** | Записывает байты в файл | `filepath string, data []byte` | `error` | - |
| **WriteStringToFile** | Записывает строку в файл | `filepath string, data string` | `error` | - |
| **ReadFile** | Читает файл или URL | `path string` | `[]byte, error` | Универсальное чтение |
| **ChunkRead** | Читает блок файла | `filepath string, offset, size int64` | `[]string, error` | По строкам |
| **ParallelChunkRead** | Параллельное чтение блоков | `filepath string, chunkSize int64` | `<-chan []string, error` | Канал строк |

---

## 8. СЕТЕВЫЕ ФУНКЦИИ (Netutil)

### Пакет: `github.com/duke-git/lancet/v2/netutil`

| Функция | Описание | Параметры | Возвращает | Применение |
|---------|----------|-----------|------------|------------|
| **ConvertMapToQueryString** | Map в query string | `param map[string]interface{}` | `string` | URL параметры |
| **EncodeUrl** | Кодирует URL | `urlStr string` | `string` | URL encoding |
| **GetInternalIp** | Внутренний IP | `()` | `string, error` | IPv4 |
| **GetIps** | Все IP адреса | `()` | `[]string, error` | IPv4 системы |
| **GetMacAddrs** | MAC адреса | `()` | `[]string, error` | Сетевых интерфейсов |
| **GetPublicIpInfo** | Информация о публичном IP | `()` | `(*PublicIpInfo, error)` | Из ip-api.com |
| **GetRequestPublicIp** | IP из HTTP запроса | `req *http.Request` | `string` | X-Forwarded-For и др. |
| **IsPublicIP** | Проверка публичного IP | `ip net.IP` | `bool` | - |
| **IsInternalIP** | Проверка внутреннего IP | `ip net.IP` | `bool` | - |
| **HttpRequest** | HTTP запрос | `method, url string, ...options` | `*HttpRequest` | Билдер запроса |
| **HttpClient** | HTTP клиент | `()` | `*HttpClient` | Обертка http.Client |
| **SendRequest** | Отправка HTTP запроса | `req *HttpRequest` | `(*http.Response, error)` | - |
| **DecodeResponse** | Декодирование ответа | `resp *http.Response, obj interface{}` | `error` | JSON/XML |
| **StructToUrlValues** | Struct в URL values | `structObj interface{}` | `url.Values, error` | - |
| **DownloadFile** | Скачивание файла | `url, filepath string` | `error` | - |
| **UploadFile** | Загрузка файла | `url, filepath string` | `(*http.Response, error)` | - |
| **IsPingConnected** | Проверка ping | `host string` | `bool` | ICMP ping |
| **IsTelnetConnected** | Проверка telnet | `host string, port int` | `bool` | TCP соединение |
| **BuildUrl** | Создание URL | `base string, path string, query map[string]interface{}` | `string` | - |
| **AddQueryParams** | Добавление параметров | `baseUrl string, params map[string]interface{}` | `string` | - |

---

## 9. КОНКУРЕНТНОСТЬ (Concurrency)

### Пакет: `github.com/duke-git/lancet/v2/concurrency`

| Функция | Описание | Параметры | Возвращает | Использование |
|---------|----------|-----------|------------|---------------|
| **NewChannel** | Создает Channel | `()` | `*Channel[T]` | Обертка над каналом |
| **Bridge** | Объединяет каналы | `ctx context.Context, chanStream <-chan <-chan T` | `<-chan T` | - |
| **FanIn** | Слияние каналов | `ctx context.Context, channels ...<-chan T` | `<-chan T` | N в 1 |
| **Generate** | Генерирует значения в канал | `ctx context.Context, values ...T` | `<-chan T` | - |
| **Or** | Закрывается при закрытии любого | `channels ...<-chan interface{}` | `<-chan interface{}` | OR логика |
| **OrDone** | Канал до отмены контекста | `ctx context.Context, c <-chan T` | `<-chan T` | - |
| **Repeat** | Повторяет значения | `ctx context.Context, values ...T` | `<-chan T` | Бесконечно |
| **RepeatFn** | Повторяет функцию | `ctx context.Context, fn func() T` | `<-chan T` | Бесконечно |
| **Take** | Берет N значений | `ctx context.Context, c <-chan T, num int` | `<-chan T` | Ограничение |
| **Tee** | Разделяет канал | `ctx context.Context, c <-chan T` | `(<-chan T, <-chan T)` | 1 в 2 |
| **NewKeyedLocker** | Создает блокировщик по ключу | `()` | `*KeyedLocker[K]` | Мютекс по ключу |
| **Do** | Выполняет с блокировкой | `key K, fn func()` | `void` | Атомарно по ключу |
| **NewRWKeyedLocker** | RW блокировщик по ключу | `()` | `*RWKeyedLocker[K]` | RWMutex по ключу |
| **RLock** | Блокировка чтения | `key K, fn func()` | `void` | - |
| **Lock** | Блокировка записи | `key K, fn func()` | `void` | - |
| **NewTryKeyedLocker** | Неблокирующий блокировщик | `()` | `*TryKeyedLocker[K]` | TryLock по ключу |
| **TryLock** | Попытка блокировки | `key K` | `bool` | Не блокирует |
| **Unlock** | Разблокировка | `key K` | `void` | - |

---

## 10. ВАЛИДАЦИЯ (Validator)

### Пакет: `github.com/duke-git/lancet/v2/validator`

| Функция | Описание | Параметры | Возвращает | Паттерн |
|---------|----------|-----------|------------|---------|
| **ContainChinese** | Содержит китайские символы | `s string` | `bool` | [\p{Han}] |
| **ContainLetter** | Содержит буквы | `s string` | `bool` | [a-zA-Z] |
| **ContainLower** | Содержит строчные | `s string` | `bool` | [a-z] |
| **ContainUpper** | Содержит заглавные | `s string` | `bool` | [A-Z] |
| **IsAlpha** | Только буквы | `s string` | `bool` | ^[a-zA-Z]+$ |
| **IsAllUpper** | Все заглавные | `s string` | `bool` | ^[A-Z]+$ |
| **IsAllLower** | Все строчные | `s string` | `bool` | ^[a-z]+$ |
| **IsBase64** | Base64 строка | `s string` | `bool` | Base64 алфавит |
| **IsChineseMobile** | Китайский мобильный | `s string` | `bool` | 中国手机号 |
| **IsChineseIdNum** | Китайский ID | `s string` | `bool` | 身份证号 |
| **IsChinesePhone** | Китайский телефон | `s string` | `bool` | xxx-xxxxxxxx |
| **IsCreditCard** | Кредитная карта | `s string` | `bool` | Luhn алгоритм |
| **IsDns** | DNS имя | `s string` | `bool` | RFC 1035 |
| **IsEmail** | Email адрес | `s string` | `bool` | RFC 5322 |
| **IsEmptyString** | Пустая строка | `s string` | `bool` | len(s) == 0 |
| **IsFloat** | Значение float | `v interface{}` | `bool` | float32/64 |
| **IsFloatStr** | Строка float | `s string` | `bool` | Парсится в float |
| **IsNumber** | Числовое значение | `v interface{}` | `bool` | int/uint/float |
| **IsNumberStr** | Строка числа | `s string` | `bool` | Парсится в число |
| **IsAlphaNumeric** | Буквы и цифры | `s string` | `bool` | ^[a-zA-Z0-9]+$ |
| **IsJSON** | JSON строка | `s string` | `bool` | Валидный JSON |
| **IsRegexMatch** | Соответствует regex | `s, pattern string` | `bool` | - |
| **IsInt** | Целое число | `v interface{}` | `bool` | int типы |
| **IsIntStr** | Строка целого | `s string` | `bool` | Парсится в int |
| **IsIp** | IP адрес | `s string` | `bool` | IPv4/IPv6 |
| **IsIpV4** | IPv4 адрес | `s string` | `bool` | xxx.xxx.xxx.xxx |
| **IsIpV6** | IPv6 адрес | `s string` | `bool` | xxxx:xxxx::xxxx |
| **IsIpPort** | IP:Port | `s string` | `bool` | host:port |
| **IsStrongPassword** | Сильный пароль | `s string, length int` | `bool` | Заглавные+строчные+цифры+символы |
| **IsUrl** | URL адрес | `s string` | `bool` | RFC 3986 |
| **IsWeakPassword** | Слабый пароль | `s string, length int` | `bool` | Только буквы или цифры |
| **IsZeroValue** | Нулевое значение | `v interface{}` | `bool` | Zero value типа |
| **IsGBK** | GBK кодировка | `data []byte` | `bool` | - |
| **IsASCII** | ASCII символы | `s string` | `bool` | 0-127 |
| **IsPrintable** | Печатаемые символы | `s string` | `bool` | unicode.IsPrint |
| **IsBin** | Бинарная строка | `s string` | `bool` | ^[01]+$ |
| **IsHex** | Hex строка | `s string` | `bool` | ^[0-9a-fA-F]+$ |
| **IsBase64URL** | Base64 URL безопасная | `s string` | `bool` | URL-safe алфавит |
| **IsJWT** | JWT токен | `s string` | `bool` | header.payload.signature |
| **IsVisa** | Visa карта | `s string` | `bool` | 4xxx xxxx xxxx xxxx |
| **IsMasterCard** | MasterCard | `s string` | `bool` | 5xxx xxxx xxxx xxxx |
| **IsAmericanExpress** | AmEx карта | `s string` | `bool` | 3xxx xxxxxx xxxxx |
| **IsUnionPay** | UnionPay карта | `s string` | `bool` | 62xx xxxx xxxx xxxx |
| **IsChinaUnionPay** | China UnionPay | `s string` | `bool` | 中国银联 |

---

## 11. ПРЕОБРАЗОВАНИЕ ТИПОВ (Convertor)

### Пакет: `github.com/duke-git/lancet/v2/convertor`

| Функция | Описание | Параметры | Возвращает | Примечания |
|---------|----------|-----------|------------|------------|
| **ColorHexToRGB** | Hex в RGB | `hexColor string` | `(int, int, int, error)` | #FFFFFF -> 255,255,255 |
| **ColorRGBToHex** | RGB в Hex | `r, g, b int` | `string` | 255,255,255 -> #FFFFFF |
| **ToBool** | В boolean | `s string` | `bool, error` | "true", "1", "yes" |
| **ToBytes** | В []byte | `v interface{}` | `[]byte, error` | - |
| **ToChar** | В []rune | `s string` | `[]rune` | Unicode символы |
| **ToChannel** | В канал | `slice []T` | `<-chan T` | Read-only канал |
| **ToFloat** | В float64 | `v interface{}` | `float64, error` | - |
| **ToInt** | В int64 | `v interface{}` | `int64, error` | - |
| **ToJson** | В JSON строку | `v interface{}` | `string, error` | - |
| **ToMap** | Slice в map | `slice []T, iteratee func` | `map[K]T` | По функции |
| **ToPointer** | В указатель | `v T` | `*T` | - |
| **ToString** | В строку | `v interface{}` | `string` | - |
| **StructToMap** | Struct в map | `s interface{}` | `map[string]interface{}` | Только экспортируемые поля |
| **MapToSlice** | Map в slice | `m map[K]V, iteratee func` | `[]T` | По функции |
| **EncodeByte** | Кодирование в байты | `data interface{}` | `[]byte, error` | gob кодирование |
| **DecodeByte** | Декодирование из байтов | `data []byte, target interface{}` | `error` | gob декодирование |
| **DeepClone** | Глубокое копирование | `src T` | `T` | Рекурсивно |
| **CopyProperties** | Копирование полей | `src, dest interface{}` | `error` | По именам полей |
| **ToInterface** | Reflect в interface{} | `v reflect.Value` | `interface{}` | - |
| **Utf8ToGbk** | UTF-8 в GBK | `data []byte` | `[]byte, error` | Китайская кодировка |
| **GbkToUtf8** | GBK в UTF-8 | `data []byte` | `[]byte, error` | - |
| **ToStdBase64** | В стандартный Base64 | `data []byte` | `string` | Standard encoding |
| **ToUrlBase64** | В URL Base64 | `data []byte` | `string` | URL-safe encoding |
| **ToRawStdBase64** | В raw стандартный Base64 | `data []byte` | `string` | Без padding |
| **ToRawUrlBase64** | В raw URL Base64 | `data []byte` | `string` | Без padding |
| **ToBigInt** | В *big.Int | `v interface{}` | `*big.Int, error` | Большие числа |

---

## 12. РАБОТА С MAP (Maputil)

### Пакет: `github.com/duke-git/lancet/v2/maputil`

| Функция | Описание | Параметры | Возвращает | Особенности |
|---------|----------|-----------|------------|-------------|
| **MapTo** | Быстрое преобразование в struct | `src, dest interface{}` | `error` | Reflection |
| **ForEach** | Итерация по map | `m map[K]V, iteratee func` | `void` | - |
| **Filter** | Фильтрация map | `m map[K]V, predicate func` | `map[K]V` | - |
| **FilterByKeys** | Фильтр по ключам | `m map[K]V, keys []K` | `map[K]V` | - |
| **FilterByValues** | Фильтр по значениям | `m map[K]V, values []V` | `map[K]V` | - |
| **OmitBy** | Исключение по предикату | `m map[K]V, predicate func` | `map[K]V` | Противоположность Filter |
| **OmitByKeys** | Исключение по ключам | `m map[K]V, keys []K` | `map[K]V` | - |
| **OmitByValues** | Исключение по значениям | `m map[K]V, values []V` | `map[K]V` | - |
| **Intersect** | Пересечение maps | `maps ...map[K]V` | `map[K]V` | Общие ключи |
| **Keys** | Все ключи | `m map[K]V` | `[]K` | - |
| **KeysBy** | Ключи через функцию | `m map[K]V, mapper func` | `[]T` | - |
| **Merge** | Слияние maps | `maps ...map[K]V` | `map[K]V` | Последний побеждает |
| **Minus** | Разность maps | `mapA, mapB map[K]V` | `map[K]V` | A - B |
| **Values** | Все значения | `m map[K]V` | `[]V` | - |
| **ValuesBy** | Значения через функцию | `m map[K]V, mapper func` | `[]T` | - |
| **MapKeys** | Преобразование ключей | `m map[K]V, mapper func` | `map[T]V` | - |
| **MapValues** | Преобразование значений | `m map[K]V, mapper func` | `map[K]T` | - |
| **Entries** | В массив пар | `m map[K]V` | `[]Entry[K,V]` | key-value пары |
| **FromEntries** | Из массива пар | `entries []Entry[K,V]` | `map[K]V` | - |
| **Transform** | Полное преобразование | `m map[K]V, transformer func` | `map[A]B` | - |
| **IsDisjoint** | Нет общих ключей | `mapA, mapB map[K]V` | `bool` | - |
| **HasKey** | Есть ключ | `m map[K]V, key K` | `bool` | - |
| **GetOrSet** | Получить или установить | `m map[K]V, key K, value V` | `V` | - |
| **MapToStruct** | Map в struct | `m map[string]interface{}, s interface{}` | `error` | По тегам |
| **ToSortedSlicesDefault** | В отсортированные слайсы | `m map[K]V` | `[]K, []V` | По ключам |
| **ToSortedSlicesWithComparator** | С компаратором | `m map[K]V, less func` | `[]K, []V` | - |
| **NewOrderedMap** | Создает упорядоченную map | `()` | `*OrderedMap[K,V]` | Сохраняет порядок |
| **NewConcurrentMap** | Создает конкурентную map | `shardCount uint64` | `*ConcurrentMap[K,V]` | Thread-safe |
| **SortByKey** | Сортировка по ключу | `m map[K]V` | `map[K]V` | Новая map |
| **GetOrDefault** | Получить или дефолт | `m map[K]V, key K, defaultValue V` | `V` | - |
| **FindValuesBy** | Найти значения по условию | `m map[K]V, predicate func` | `[]V` | - |

---

## 13. СЛУЧАЙНЫЕ ЗНАЧЕНИЯ (Random)

### Пакет: `github.com/duke-git/lancet/v2/random`

| Функция | Описание | Параметры | Возвращает | Диапазон |
|---------|----------|-----------|------------|----------|
| **RandBytes** | Случайные байты | `length int` | `[]byte` | 0-255 |
| **RandInt** | Случайное число | `min, max int` | `int` | [min, max) |
| **RandString** | Случайная строка | `length int` | `string` | a-zA-Z0-9 |
| **RandUpper** | Заглавные буквы | `length int` | `string` | A-Z |
| **RandLower** | Строчные буквы | `length int` | `string` | a-z |
| **RandNumeral** | Цифры | `length int` | `string` | 0-9 |
| **RandNumeralOrLetter** | Цифры или буквы | `length int` | `string` | a-zA-Z0-9 |
| **UUIdV4** | UUID v4 | `()` | `string` | RFC 4122 |
| **RandUniqueIntSlice** | Уникальные числа | `length, min, max int` | `[]int` | Без повторов |
| **RandSymbolChar** | Символы | `length int` | `string` | !@#$%^&* |
| **RandFloat** | Случайный float | `min, max float64, precision int` | `float64` | [min, max) |
| **RandFloats** | Слайс float | `length int, min, max float64, precision int` | `[]float64` | Уникальные |
| **RandStringSlice** | Слайс строк | `length, strLen int, charset string` | `[]string` | - |
| **RandBool** | Случайный bool | `()` | `bool` | true/false |
| **RandBoolSlice** | Слайс bool | `length int` | `[]bool` | - |
| **RandIntSlice** | Слайс int | `length, min, max int` | `[]int` | Могут повторяться |
| **RandFromGivenSlice** | Элемент из слайса | `slice []T` | `T` | - |
| **RandSliceFromGivenSlice** | Подслайс | `slice []T, num int` | `[]T` | Случайные элементы |
| **RandNumberOfLength** | Число заданной длины | `length int` | `int` | Первая цифра != 0 |

---

## 14. ФУНКЦИОНАЛЬНОЕ ПРОГРАММИРОВАНИЕ (Function)

### Пакет: `github.com/duke-git/lancet/v2/function`

| Функция | Описание | Параметры | Возвращает | Применение |
|---------|----------|-----------|------------|------------|
| **After** | Выполнить после N вызовов | `n int, fn func` | `func` | Ленивое выполнение |
| **Before** | Выполнить до N вызовов | `n int, fn func` | `func` | Ограничение вызовов |
| **CurryFn** | Каррирование функции | `fn func` | `func` | Частичное применение |
| **Compose** | Композиция функций | `fns ...func` | `func` | Справа налево |
| **Delay** | Задержка выполнения | `duration time.Duration, fn func` | `void` | - |
| **Debounce** | Дебаунсинг | `duration time.Duration, fn func` | `func` | Предотвращает частые вызовы |
| **Throttle** | Троттлинг | `duration time.Duration, fn func` | `func` | Ограничивает частоту |
| **Schedule** | Планировщик | `duration time.Duration, fn func` | `chan bool` | Периодическое выполнение |
| **Pipeline** | Конвейер функций | `fns ...func` | `func` | Слева направо |
| **AcceptIf** | Условное выполнение | `predicate func, fn func` | `func` | С проверкой |
| **And** | Логическое И предикатов | `predicates ...func` | `func` | Все true |
| **Or** | Логическое ИЛИ предикатов | `predicates ...func` | `func` | Любой true |
| **Negate** | Отрицание предиката | `predicate func` | `func` | !predicate |
| **Nor** | Логическое НЕ-ИЛИ | `predicates ...func` | `func` | Все false |
| **Nand** | Логическое НЕ-И | `predicates ...func` | `func` | Не все true |
| **Xnor** | Логическое НЕ-исключающее ИЛИ | `predicates ...func` | `func` | Одинаковые значения |
| **Watcher** | Измеритель времени | `()` | `*Watcher` | Start/Stop/Reset |

---

## 15. ОБРАБОТКА ОШИБОК (Xerror)

### Пакет: `github.com/duke-git/lancet/v2/xerror`

| Функция | Описание | Параметры | Возвращает | Особенности |
|---------|----------|-----------|------------|-------------|
| **New** | Создает XError | `message string` | `*XError` | С трассировкой стека |
| **Wrap** | Обертывает ошибку | `err error, message string` | `*XError` | Добавляет контекст |
| **Unwrap** | Извлекает XError | `err error` | `*XError` | errors.As |
| **XError_Wrap** | Метод обертывания | `message string` | `*XError` | Копирует ID |
| **XError_Unwrap** | Совместимость с pkg/errors | `()` | `error` | - |
| **XError_With** | Добавляет key-value | `key string, value interface{}` | `*XError` | Контекст |
| **XError_Id** | Устанавливает ID | `id string` | `*XError` | Для сравнения |
| **XError_Is** | Проверяет ID | `target error` | `bool` | Сравнение ID |
| **XError_Values** | Возвращает values | `()` | `map[string]interface{}` | Все key-value |
| **XError_StackTrace** | Трассировка стека | `()` | `[]uintptr` | pkg/errors совместимость |
| **XError_Info** | Информация об ошибке | `()` | `map[string]interface{}` | Печатаемая |
| **XError_Error** | Стандартный интерфейс error | `()` | `string` | - |
| **TryUnwrap** | Разворачивает или паникует | `val T, err error` | `T` | Упрощает обработку |
| **TryCatch** | Try-catch симуляция | `try func, catch func` | `void` | Java-style |

---

## Заключение

Библиотека Lancet предоставляет более **700 утилитарных функций**, охватывающих практически все аспекты разработки на Go:

### Основные преимущества:
- ✅ **Полное покрытие тестами**
- ✅ **Поддержка дженериков** (Go 1.18+)
- ✅ **Минимальные зависимости**
- ✅ **Высокая производительность**
- ✅ **Простота использования**

### Области применения:
- Обработка данных (слайсы, строки, числа)
- Сетевое программирование и HTTP
- Криптография и безопасность
- Работа с файлами и форматами
- Конкурентное программирование
- Функциональное программирование
- Валидация и преобразование данных

Lancet является отличным выбором для разработчиков Go, которые хотят избежать написания повторяющегося кода и сосредоточиться на бизнес-логике своих приложений.

Я изучу репозиторий Lancet и предоставлю вам подробную документацию по всем функциям. Давайте начнем с получения информации с GitHub.Теперь давайте получим более подробную информацию о каждом пакете. Начну с загрузки документации для основных пакетов:Я создам подробную документацию по всем функциям Lancet в табличном виде, сгруппированную по назначению, на основе полученной информации:Я создал подробную документацию по библиотеке Lancet Go в табличном виде, сгруппированную по функциональному назначению. Документация включает:

## Основные разделы:

1. **Алгоритмы** - сортировка, поиск, кэширование
2. **Работа со слайсами** - более 60 функций для манипуляций с массивами
3. **Строковые операции** - преобразования, валидация, форматирование
4. **Математические функции** - статистика, тригонометрия, комбинаторика
5. **Дата и время** - форматирование, арифметика, часовые пояса
6. **Криптография** - шифрование AES/DES/RSA, хэширование, подписи
7. **Файловые операции** - чтение, запись, архивирование
8. **Сетевые функции** - HTTP клиент, IP операции, URL манипуляции
9. **Конкурентность** - каналы, блокировки, синхронизация
10. **Валидация** - проверка email, IP, карт, паролей
11. **Преобразование типов** - конвертация между различными типами данных
12. **Работа с Map** - фильтрация, преобразования, сортировка
13. **Случайные значения** - генерация UUID, паролей, чисел
14. **Функциональное программирование** - каррирование, композиция, дебаунсинг
15. **Обработка ошибок** - расширенные возможности для работы с ошибками

## Ключевые особенности библиотеки:

- **700+ функций** с полным покрытием тестами
- **Поддержка дженериков** для Go 1.18+
- **Минимальные зависимости** - только стандартная библиотека
- **Модульная архитектура** - импортируйте только нужные пакеты
- **Высокая производительность** - оптимизированные алгоритмы

Эта документация поможет вам быстро найти нужную функцию и понять, как её использовать в ваших Go проектах. Каждая таблица содержит подробное описание параметров, возвращаемых значений и практических применений.

Создам полные биндинги всех функций библиотеки Lancet для использования в JavaScript через goja:Я создал полные биндинги всех функций библиотеки Lancet для JavaScript Runtime (goja). Код включает:

## Основные особенности:

### 🎯 **Полное покрытие**
- **700+ функций** из всех 24 пакетов Lancet
- **Модульная организация** - каждый пакет в отдельном namespace
- **Типизированные биндинги** с поддержкой дженериков

### 📦 **Структура биндингов**
- `$algorithm` - алгоритмы сортировки и поиска
- `$slice` - операции с массивами
- `$strutil` - строковые функции
- `$mathutil` - математические операции
- `$datetime` - работа с датой/временем
- `$cryptor` - криптографические функции
- `$fileutil` - файловые операции
- `$netutil` - сетевые функции
- `$validator` - валидация данных
- И многие другие...

### 💡 **Примеры использования в JavaScript**:

```javascript
// Строковые операции
let camelCase = $strutil.camelCase("hello_world"); // "helloWorld"
let reversed = $strutil.reverse("hello"); // "olleh"

// Операции с массивами
let numbers = [1, 2, 3, 4, 5];
let doubled = $slice.map(numbers, x => x * 2); // [2, 4, 6, 8, 10]
let unique = $slice.unique([1, 2, 2, 3, 3]); // [1, 2, 3]

// Валидация
let isValidEmail = $validator.isEmail("test@example.com"); // true
let isStrongPwd = $validator.isStrongPassword("MyPass123!", 8); // true

// Математические операции
let average = $mathutil.average(1, 2, 3, 4, 5); // 3
let factorial = $mathutil.factorial(5); // 120

// Криптография
let hash = $cryptor.md5String("hello world");
let encrypted = $cryptor.aesEcbEncrypt(data, key);

// Случайные значения
let randomStr = $random.randString(10);
let uuid = $random.uuIdV4();

// Работа с файлами
let content = $fileutil.readFileToString("file.txt");
$fileutil.writeStringToFile("output.txt", "Hello World");
```

### 🚀 **Использование**:

```go
func main() {
    vm := goja.New()
    InitLancetBindings(vm) // Инициализация всех биндингов
    
    // Теперь все функции Lancet доступны в JavaScript
    vm.RunString(`
        let result = $strutil.camelCase("my_variable_name");
        console.log(result); // "myVariableName"
    `)
}
```

### ⚡ **Преимущества**:

1. **Полная совместимость** - все функции Lancet доступны в JS
2. **Типобезопасность** - правильная обработка типов Go в JS
3. **Простота использования** - интуитивные имена функций
4. **Модульность** - можно использовать только нужные пакеты
5. **Производительность** - прямые вызовы Go функций

Теперь вы можете использовать всю мощь библиотеки Lancet в JavaScript коде через goja runtime!