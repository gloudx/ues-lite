# Документация по библиотеке lo для Go

**lo** - это библиотека утилит для Go 1.18+, основанная на дженериках, которая предоставляет множество удобных методов в стиле Lodash для работы со слайсами, картами, строками, каналами и функциями.

## Установка

```bash
go get github.com/samber/lo@v1
```

## Импорт

```go
import (
    "github.com/samber/lo"
    lop "github.com/samber/lo/parallel"  // для параллельной обработки
    lom "github.com/samber/lo/mutable"   // для мутирующих операций
)
```

---

## 1. Функции для работы со слайсами

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **Filter** | Фильтрует элементы по предикату | `Filter[T]([]T, func(T, int) bool) []T` | `lo.Filter([]int{1,2,3,4}, func(x int, i int) bool { return x%2 == 0 })` |
| **Map** | Преобразует элементы слайса | `Map[T, R]([]T, func(T, int) R) []R` | `lo.Map([]int{1,2,3}, func(x int, i int) string { return strconv.Itoa(x) })` |
| **UniqMap** | Преобразует и оставляет уникальные значения | `UniqMap[T, R]([]T, func(T, int) R) []R` | `lo.UniqMap(users, func(u User, i int) string { return u.Name })` |
| **FilterMap** | Фильтрует и преобразует одновременно | `FilterMap[T, R]([]T, func(T, int) (R, bool)) []R` | `lo.FilterMap([]string{"cpu", "gpu"}, func(x string, _ int) (string, bool) { return "x"+x, strings.HasSuffix(x, "pu") })` |
| **FlatMap** | Преобразует и выравнивает слайс | `FlatMap[T, R]([]T, func(T, int) []R) []R` | `lo.FlatMap([]int{1,2}, func(x int, _ int) []string { return []string{strconv.Itoa(x)} })` |
| **Reduce** | Сворачивает слайс к одному значению | `Reduce[T, R]([]T, func(R, T, int) R, R) R` | `lo.Reduce([]int{1,2,3}, func(agg int, item int, _ int) int { return agg + item }, 0)` |
| **ReduceRight** | Сворачивает слайс справа налево | `ReduceRight[T, R]([]T, func(R, T, int) R, R) R` | `lo.ReduceRight([][]int{{0,1}, {2,3}}, func(agg []int, item []int, _ int) []int { return append(agg, item...) }, []int{})` |
| **ForEach** | Выполняет функцию для каждого элемента | `ForEach[T]([]T, func(T, int))` | `lo.ForEach([]string{"hello", "world"}, func(x string, _ int) { println(x) })` |
| **ForEachWhile** | Выполняет функцию пока условие истинно | `ForEachWhile[T]([]T, func(T, int) bool)` | `lo.ForEachWhile([]int{1,2,-1,3}, func(x int, _ int) bool { return x > 0 })` |
| **Times** | Создает слайс, вызывая функцию N раз | `Times[T](int, func(int) T) []T` | `lo.Times(3, func(i int) string { return strconv.Itoa(i) })` |
| **Uniq** | Возвращает уникальные элементы | `Uniq[T]([]T) []T` | `lo.Uniq([]int{1,2,2,1})` → `[]int{1,2}` |
| **UniqBy** | Возвращает уникальные элементы по критерию | `UniqBy[T, U]([]T, func(T) U) []T` | `lo.UniqBy([]int{0,1,2,3,4,5}, func(i int) int { return i%3 })` |
| **GroupBy** | Группирует элементы по ключу | `GroupBy[T, U]([]T, func(T) U) map[U][]T` | `lo.GroupBy([]int{0,1,2,3,4,5}, func(i int) int { return i%3 })` |
| **GroupByMap** | Группирует с преобразованием значений | `GroupByMap[T, U, R]([]T, func(T) (U, R)) map[U][]R` | `lo.GroupByMap([]int{0,1,2}, func(i int) (int, int) { return i%2, i*2 })` |
| **Chunk** | Разделяет слайс на части заданного размера | `Chunk[T]([]T, int) [][]T` | `lo.Chunk([]int{0,1,2,3,4,5}, 2)` → `[][]int{{0,1}, {2,3}, {4,5}}` |
| **PartitionBy** | Разделяет слайс на группы по критерию | `PartitionBy[T, K]([]T, func(T) K) [][]T` | `lo.PartitionBy([]int{-2,-1,0,1,2}, func(x int) string { if x < 0 { return "neg" } return "pos" })` |
| **Flatten** | Выравнивает двумерный слайс | `Flatten[T]([][]T) []T` | `lo.Flatten([][]int{{0,1}, {2,3,4,5}})` → `[]int{0,1,2,3,4,5}` |
| **Interleave** | Переплетает несколько слайсов | `Interleave[T](...[]T) []T` | `lo.Interleave([]int{1,4,7}, []int{2,5,8}, []int{3,6,9})` |
| **Shuffle** | Перемешивает элементы слайса | `Shuffle[T]([]T) []T` | `lo.Shuffle([]int{0,1,2,3,4,5})` |
| **Reverse** | Переворачивает слайс | `Reverse[T]([]T) []T` | `lo.Reverse([]int{0,1,2,3})` → `[]int{3,2,1,0}` |
| **Fill** | Заполняет слайс значением | `Fill[T]([]T, T) []T` | `lo.Fill([]int{1,2,3}, 42)` → `[]int{42,42,42}` |
| **Repeat** | Создает слайс с N копиями значения | `Repeat[T](int, T) []T` | `lo.Repeat(3, "hello")` → `[]string{"hello","hello","hello"}` |
| **RepeatBy** | Создает слайс вызовом функции N раз | `RepeatBy[T](int, func(int) T) []T` | `lo.RepeatBy(3, func(i int) int { return i*i })` |
| **KeyBy** | Создает мапу из слайса по ключу | `KeyBy[T, K]([]T, func(T) K) map[K]T` | `lo.KeyBy([]string{"a","aa","aaa"}, func(str string) int { return len(str) })` |
| **SliceToMap** | Преобразует слайс в мапу | `SliceToMap[T, K, V]([]T, func(T) (K, V)) map[K]V` | `lo.SliceToMap(users, func(u User) (string, int) { return u.Name, u.Age })` |
| **FilterSliceToMap** | Преобразует слайс в мапу с фильтрацией | `FilterSliceToMap[T, K, V]([]T, func(T) (K, V, bool)) map[K]V` | `lo.FilterSliceToMap([]string{"a","aa"}, func(s string) (string, int, bool) { return s, len(s), len(s) > 1 })` |
| **Keyify** | Создает set из слайса | `Keyify[T]([]T) map[T]struct{}` | `lo.Keyify([]int{1,1,2,3})` → `map[int]struct{}{1:{}, 2:{}, 3:{}}` |
| **Drop** | Удаляет N элементов с начала | `Drop[T]([]T, int) []T` | `lo.Drop([]int{0,1,2,3,4,5}, 2)` → `[]int{2,3,4,5}` |
| **DropRight** | Удаляет N элементов с конца | `DropRight[T]([]T, int) []T` | `lo.DropRight([]int{0,1,2,3,4,5}, 2)` → `[]int{0,1,2,3}` |
| **DropWhile** | Удаляет элементы с начала пока условие истинно | `DropWhile[T]([]T, func(T) bool) []T` | `lo.DropWhile([]string{"a","aa","aaa"}, func(s string) bool { return len(s) <= 2 })` |
| **DropRightWhile** | Удаляет элементы с конца пока условие истинно | `DropRightWhile[T]([]T, func(T) bool) []T` | `lo.DropRightWhile([]string{"a","aa","aaa"}, func(s string) bool { return len(s) <= 2 })` |
| **DropByIndex** | Удаляет элементы по индексам | `DropByIndex[T]([]T, ...int) []T` | `lo.DropByIndex([]int{0,1,2,3,4,5}, 2, 4, -1)` |
| **Reject** | Отклоняет элементы по предикату | `Reject[T]([]T, func(T, int) bool) []T` | `lo.Reject([]int{1,2,3,4}, func(x int, _ int) bool { return x%2 == 0 })` |
| **RejectMap** | Отклоняет и преобразует | `RejectMap[T, R]([]T, func(T, int) (R, bool)) []R` | `lo.RejectMap([]int{1,2,3,4}, func(x int, _ int) (int, bool) { return x*10, x%2 == 0 })` |
| **FilterReject** | Разделяет на два слайса по предикату | `FilterReject[T]([]T, func(T, int) bool) ([]T, []T)` | `kept, rejected := lo.FilterReject([]int{1,2,3,4}, func(x int, _ int) bool { return x%2 == 0 })` |
| **Count** | Подсчитывает количество значений | `Count[T]([]T, T) int` | `lo.Count([]int{1,5,1}, 1)` → `2` |
| **CountBy** | Подсчитывает элементы по предикату | `CountBy[T]([]T, func(T) bool) int` | `lo.CountBy([]int{1,5,1}, func(i int) bool { return i < 4 })` |
| **CountValues** | Подсчитывает количество каждого значения | `CountValues[T]([]T) map[T]int` | `lo.CountValues([]int{1,2,2})` → `map[int]int{1:1, 2:2}` |
| **CountValuesBy** | Подсчитывает значения по критерию | `CountValuesBy[T, U]([]T, func(T) U) map[U]int` | `lo.CountValuesBy([]int{1,2,3}, func(i int) bool { return i%2==0 })` |
| **Subset** | Возвращает подмножество слайса | `Subset[T]([]T, int, int) []T` | `lo.Subset([]int{0,1,2,3,4}, 2, 3)` → `[]int{2,3,4}` |
| **Slice** | Возвращает срез слайса | `Slice[T]([]T, int, int) []T` | `lo.Slice([]int{0,1,2,3,4}, 1, 3)` → `[]int{1,2}` |
| **Replace** | Заменяет элементы | `Replace[T]([]T, T, T, int) []T` | `lo.Replace([]int{0,1,0,1}, 0, 42, 1)` |
| **ReplaceAll** | Заменяет все вхождения | `ReplaceAll[T]([]T, T, T) []T` | `lo.ReplaceAll([]int{0,1,0,1}, 0, 42)` |
| **Compact** | Удаляет нулевые значения | `Compact[T]([]T) []T` | `lo.Compact([]string{"", "foo", "", "bar"})` → `[]string{"foo", "bar"}` |
| **IsSorted** | Проверяет отсортированность | `IsSorted[T]([]T) bool` | `lo.IsSorted([]int{1,2,3,4})` → `true` |
| **IsSortedByKey** | Проверяет отсортированность по ключу | `IsSortedByKey[T, K]([]T, func(T) K) bool` | `lo.IsSortedByKey([]string{"a","bb","ccc"}, func(s string) int { return len(s) })` |
| **Splice** | Вставляет элементы по индексу | `Splice[T]([]T, int, ...T) []T` | `lo.Splice([]string{"a","b"}, 1, "1", "2")` → `[]string{"a","1","2","b"}` |

---

## 2. Функции для работы с картами

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **Keys** | Извлекает ключи из карт | `Keys[K, V](map[K]V) []K` | `lo.Keys(map[string]int{"foo":1, "bar":2})` |
| **UniqKeys** | Извлекает уникальные ключи из нескольких карт | `UniqKeys[K, V](...map[K]V) []K` | `lo.UniqKeys(map1, map2)` |
| **HasKey** | Проверяет наличие ключа | `HasKey[K, V](map[K]V, K) bool` | `lo.HasKey(map[string]int{"foo":1}, "foo")` → `true` |
| **Values** | Извлекает значения из карт | `Values[K, V](map[K]V) []V` | `lo.Values(map[string]int{"foo":1, "bar":2})` |
| **UniqValues** | Извлекает уникальные значения | `UniqValues[K, V](...map[K]V) []V` | `lo.UniqValues(map1, map2)` |
| **ValueOr** | Возвращает значение или значение по умолчанию | `ValueOr[K, V](map[K]V, K, V) V` | `lo.ValueOr(map[string]int{"foo":1}, "bar", 42)` → `42` |
| **PickBy** | Фильтрует карту по предикату | `PickBy[K, V](map[K]V, func(K, V) bool) map[K]V` | `lo.PickBy(myMap, func(k string, v int) bool { return v%2 == 1 })` |
| **PickByKeys** | Фильтрует карту по ключам | `PickByKeys[K, V](map[K]V, []K) map[K]V` | `lo.PickByKeys(myMap, []string{"foo", "baz"})` |
| **PickByValues** | Фильтрует карту по значениям | `PickByValues[K, V](map[K]V, []V) map[K]V` | `lo.PickByValues(myMap, []int{1, 3})` |
| **OmitBy** | Исключает элементы по предикату | `OmitBy[K, V](map[K]V, func(K, V) bool) map[K]V` | `lo.OmitBy(myMap, func(k string, v int) bool { return v%2 == 1 })` |
| **OmitByKeys** | Исключает элементы по ключам | `OmitByKeys[K, V](map[K]V, []K) map[K]V` | `lo.OmitByKeys(myMap, []string{"foo", "baz"})` |
| **OmitByValues** | Исключает элементы по значениям | `OmitByValues[K, V](map[K]V, []V) map[K]V` | `lo.OmitByValues(myMap, []int{1, 3})` |
| **Entries** | Преобразует карту в массив пар ключ-значение | `Entries[K, V](map[K]V) []Entry[K, V]` | `lo.Entries(map[string]int{"foo":1})` |
| **FromEntries** | Создает карту из массива пар | `FromEntries[K, V]([]Entry[K, V]) map[K]V` | `lo.FromEntries([]Entry[string, int]{{Key:"foo", Value:1}})` |
| **Invert** | Инвертирует карту (ключи↔значения) | `Invert[K, V](map[K]V) map[V]K` | `lo.Invert(map[string]int{"a":1, "b":2})` → `map[int]string{1:"a", 2:"b"}` |
| **Assign** | Объединяет карты | `Assign[K, V](...map[K]V) map[K]V` | `lo.Assign(map1, map2, map3)` |
| **MapKeys** | Преобразует ключи карты | `MapKeys[K1, K2, V](map[K1]V, func(V, K1) K2) map[K2]V` | `lo.MapKeys(myMap, func(v int, k string) int { return len(k) })` |
| **MapValues** | Преобразует значения карты | `MapValues[K, V1, V2](map[K]V1, func(V1, K) V2) map[K]V2` | `lo.MapValues(myMap, func(v int, k string) string { return strconv.Itoa(v) })` |
| **MapEntries** | Преобразует пары ключ-значение | `MapEntries[K1, V1, K2, V2](map[K1]V1, func(K1, V1) (K2, V2)) map[K2]V2` | `lo.MapEntries(myMap, func(k string, v int) (int, string) { return v, k })` |
| **MapToSlice** | Преобразует карту в слайс | `MapToSlice[K, V, R](map[K]V, func(K, V) R) []R` | `lo.MapToSlice(myMap, func(k string, v int) string { return k + ":" + strconv.Itoa(v) })` |
| **FilterMapToSlice** | Преобразует карту в слайс с фильтрацией | `FilterMapToSlice[K, V, R](map[K]V, func(K, V) (R, bool)) []R` | `lo.FilterMapToSlice(myMap, func(k string, v int) (string, bool) { return k, v > 5 })` |

---

## 3. Математические функции

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **Range** | Создает последовательность чисел | `Range(int) []int` | `lo.Range(4)` → `[0,1,2,3]` |
| **RangeFrom** | Создает последовательность от начального значения | `RangeFrom[T](T, int) []T` | `lo.RangeFrom(1, 5)` → `[1,2,3,4,5]` |
| **RangeWithSteps** | Создает последовательность с шагом | `RangeWithSteps[T](T, T, T) []T` | `lo.RangeWithSteps(0, 20, 5)` → `[0,5,10,15]` |
| **Clamp** | Ограничивает число в диапазоне | `Clamp[T](T, T, T) T` | `lo.Clamp(42, -10, 10)` → `10` |
| **Sum** | Вычисляет сумму чисел | `Sum[T]([]T) T` | `lo.Sum([]int{1,2,3,4,5})` → `15` |
| **SumBy** | Вычисляет сумму по критерию | `SumBy[T, R]([]T, func(T) R) R` | `lo.SumBy([]string{"foo","bar"}, func(s string) int { return len(s) })` |
| **Product** | Вычисляет произведение чисел | `Product[T]([]T) T` | `lo.Product([]int{1,2,3,4,5})` → `120` |
| **ProductBy** | Вычисляет произведение по критерию | `ProductBy[T, R]([]T, func(T) R) R` | `lo.ProductBy([]string{"foo","bar"}, func(s string) int { return len(s) })` |
| **Mean** | Вычисляет среднее арифметическое | `Mean[T]([]T) float64` | `lo.Mean([]int{2,3,4,5})` → `3.5` |
| **MeanBy** | Вычисляет среднее по критерию | `MeanBy[T]([]T, func(T) float64) float64` | `lo.MeanBy([]string{"aa","bbb"}, func(s string) float64 { return float64(len(s)) })` |

---

## 4. Функции для работы со строками

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **RandomString** | Генерирует случайную строку | `RandomString(int, []rune) string` | `lo.RandomString(5, lo.LettersCharset)` |
| **Substring** | Извлекает подстроку | `Substring(string, int, int) string` | `lo.Substring("hello", 2, 3)` → `"llo"` |
| **ChunkString** | Разделяет строку на части | `ChunkString(string, int) []string` | `lo.ChunkString("123456", 2)` → `[]string{"12","34","56"}` |
| **RuneLength** | Возвращает количество рун в строке | `RuneLength(string) int` | `lo.RuneLength("hellô")` → `5` |
| **PascalCase** | Преобразует в PascalCase | `PascalCase(string) string` | `lo.PascalCase("hello_world")` → `"HelloWorld"` |
| **CamelCase** | Преобразует в camelCase | `CamelCase(string) string` | `lo.CamelCase("hello_world")` → `"helloWorld"` |
| **KebabCase** | Преобразует в kebab-case | `KebabCase(string) string` | `lo.KebabCase("helloWorld")` → `"hello-world"` |
| **SnakeCase** | Преобразует в snake_case | `SnakeCase(string) string` | `lo.SnakeCase("HelloWorld")` → `"hello_world"` |
| **Words** | Разделяет строку на слова | `Words(string) []string` | `lo.Words("helloWorld")` → `[]string{"hello","world"}` |
| **Capitalize** | Делает первую букву заглавной | `Capitalize(string) string` | `lo.Capitalize("heLLO")` → `"Hello"` |
| **Ellipsis** | Обрезает строку с многоточием | `Ellipsis(string, int) string` | `lo.Ellipsis("Lorem Ipsum", 5)` → `"Lo..."` |

---

## 5. Функции для работы с кортежами (Tuples)

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **T2-T9** | Создает кортеж из 2-9 значений | `T2[A, B](A, B) Tuple2[A, B]` | `lo.T2("x", 1)` → `Tuple2[string, int]{A: "x", B: 1}` |
| **Unpack2-Unpack9** | Распаковывает кортеж | `Unpack2[A, B](Tuple2[A, B]) (A, B)` | `a, b := lo.Unpack2(tuple)` |
| **Zip2-Zip9** | Объединяет слайсы в кортежи | `Zip2[A, B]([]A, []B) []Tuple2[A, B]` | `lo.Zip2([]string{"a","b"}, []int{1,2})` |
| **ZipBy2-ZipBy9** | Объединяет слайсы с преобразованием | `ZipBy2[A, B, R]([]A, []B, func(A, B) R) []R` | `lo.ZipBy2([]string{"a"}, []int{1}, func(a string, b int) string { return a+strconv.Itoa(b) })` |
| **Unzip2-Unzip9** | Разделяет слайс кортежей | `Unzip2[A, B]([]Tuple2[A, B]) ([]A, []B)` | `a, b := lo.Unzip2([]Tuple2[string, int]{{A:"a", B:1}})` |
| **UnzipBy2-UnzipBy9** | Разделяет слайс с преобразованием | `UnzipBy2[T, A, B]([]T, func(T) (A, B)) ([]A, []B)` | `a, b := lo.UnzipBy2([]string{"hello"}, func(s string) (string, int) { return s, len(s) })` |
| **CrossJoin2-CrossJoin9** | Декартово произведение слайсов | `CrossJoin2[A, B]([]A, []B) []Tuple2[A, B]` | `lo.CrossJoin2([]string{"a","b"}, []int{1,2})` |
| **CrossJoinBy2-CrossJoinBy9** | Декартово произведение с преобразованием | `CrossJoinBy2[A, B, R]([]A, []B, func(A, B) R) []R` | `lo.CrossJoinBy2([]string{"a"}, []int{1}, func(a A, b B) string { return a+"-"+strconv.Itoa(b) })` |

---

## 6. Функции для работы со временем

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **Duration** | Измеряет время выполнения функции | `Duration(func()) time.Duration` | `duration := lo.Duration(func() { time.Sleep(1*time.Second) })` |
| **Duration0-Duration10** | Измеряет время с возвратом значений | `Duration1[T](func() T) (T, time.Duration)` | `result, duration := lo.Duration1(func() string { return "hello" })` |

---

## 7. Функции для работы с каналами

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **ChannelDispatcher** | Распределяет сообщения по каналам | `ChannelDispatcher[T](chan T, int, int, DispatchingStrategy[T]) []<-chan T` | `children := lo.ChannelDispatcher(ch, 5, 10, lo.DispatchingStrategyRoundRobin[int])` |
| **SliceToChannel** | Преобразует слайс в канал | `SliceToChannel[T](int, []T) <-chan T` | `ch := lo.SliceToChannel(2, []int{1,2,3})` |
| **ChannelToSlice** | Преобразует канал в слайс | `ChannelToSlice[T](<-chan T) []T` | `slice := lo.ChannelToSlice(ch)` |
| **Generator** | Создает канал из генератора | `Generator[T](int, func(func(T))) <-chan T` | `ch := lo.Generator(2, func(yield func(int)) { yield(1); yield(2) })` |
| **Buffer** | Буферизует N элементов из канала | `Buffer[T](<-chan T, int) ([]T, int, time.Duration, bool)` | `items, length, duration, ok := lo.Buffer(ch, 100)` |
| **BufferWithTimeout** | Буферизует с таймаутом | `BufferWithTimeout[T](<-chan T, int, time.Duration) ([]T, int, time.Duration, bool)` | `items, length, duration, ok := lo.BufferWithTimeout(ch, 100, 1*time.Second)` |
| **BufferWithContext** | Буферизует с контекстом | `BufferWithContext[T](context.Context, <-chan T, int) ([]T, int, time.Duration, bool)` | `items, length, duration, ok := lo.BufferWithContext(ctx, ch, 100)` |
| **FanIn** | Объединяет несколько каналов | `FanIn[T](int, ...<-chan T) <-chan T` | `merged := lo.FanIn(100, ch1, ch2, ch3)` |
| **FanOut** | Разветвляет канал | `FanOut[T](int, int, <-chan T) []<-chan T` | `channels := lo.FanOut(5, 100, input)` |

---

## 8. Функции пересечений и множеств

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **Contains** | Проверяет наличие элемента | `Contains[T]([]T, T) bool` | `lo.Contains([]int{1,2,3}, 2)` → `true` |
| **ContainsBy** | Проверяет наличие по предикату | `ContainsBy[T]([]T, func(T) bool) bool` | `lo.ContainsBy([]int{1,2,3}, func(x int) bool { return x > 2 })` |
| **Every** | Проверяет, что все элементы подмножества содержатся | `Every[T]([]T, []T) bool` | `lo.Every([]int{1,2,3,4}, []int{2,3})` → `true` |
| **EveryBy** | Проверяет предикат для всех элементов | `EveryBy[T]([]T, func(T) bool) bool` | `lo.EveryBy([]int{2,4,6}, func(x int) bool { return x%2 == 0 })` |
| **Some** | Проверяет, что хотя бы один элемент содержится | `Some[T]([]T, []T) bool` | `lo.Some([]int{1,2,3}, []int{3,4})` → `true` |
| **SomeBy** | Проверяет предикат для хотя бы одного элемента | `SomeBy[T]([]T, func(T) bool) bool` | `lo.SomeBy([]int{1,2,3}, func(x int) bool { return x > 2 })` |
| **None** | Проверяет, что ни один элемент не содержится | `None[T]([]T, []T) bool` | `lo.None([]int{1,2,3}, []int{4,5})` → `true` |
| **NoneBy** | Проверяет, что предикат ложен для всех элементов | `NoneBy[T]([]T, func(T) bool) bool` | `lo.NoneBy([]int{1,2,3}, func(x int) bool { return x > 5 })` |
| **Intersect** | Возвращает пересечение коллекций | `Intersect[T]([]T, []T) []T` | `lo.Intersect([]int{1,2,3}, []int{2,3,4})` → `[]int{2,3}` |
| **Difference** | Возвращает разность коллекций | `Difference[T]([]T, []T) ([]T, []T)` | `left, right := lo.Difference([]int{1,2,3}, []int{2,4})` |
| **Union** | Возвращает объединение коллекций | `Union[T](...[]T) []T` | `lo.Union([]int{1,2}, []int{2,3})` → `[]int{1,2,3}` |
| **Without** | Исключает указанные значения | `Without[T]([]T, ...T) []T` | `lo.Without([]int{1,2,3}, 2)` → `[]int{1,3}` |
| **WithoutBy** | Исключает элементы по ключу | `WithoutBy[T, K]([]T, func(T) K, ...K) []T` | `lo.WithoutBy(users, getID, 2, 3)` |
| **WithoutEmpty** | Исключает пустые значения | `WithoutEmpty[T]([]T) []T` | `lo.WithoutEmpty([]int{0,1,0,2})` → `[]int{1,2}` |
| **WithoutNth** | Исключает элемент по индексу | `WithoutNth[T]([]T, ...int) []T` | `lo.WithoutNth([]int{1,2,3,4}, 1, 3)` → `[]int{1,3}` |
| **ElementsMatch** | Проверяет совпадение множеств | `ElementsMatch[T]([]T, []T) bool` | `lo.ElementsMatch([]int{1,1,2}, []int{2,1,1})` → `true` |
| **ElementsMatchBy** | Проверяет совпадение по ключу | `ElementsMatchBy[T, K]([]T, []T, func(T) K) bool` | `lo.ElementsMatchBy(users1, users2, func(u User) string { return u.ID })` |

---

## 9. Функции поиска

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **IndexOf** | Находит индекс первого вхождения | `IndexOf[T]([]T, T) int` | `lo.IndexOf([]int{1,2,3,2}, 2)` → `1` |
| **LastIndexOf** | Находит индекс последнего вхождения | `LastIndexOf[T]([]T, T) int` | `lo.LastIndexOf([]int{1,2,3,2}, 2)` → `3` |
| **Find** | Находит элемент по предикату | `Find[T]([]T, func(T) bool) (T, bool)` | `item, ok := lo.Find([]string{"a","b","c"}, func(s string) bool { return s == "b" })` |
| **FindIndexOf** | Находит элемент и индекс по предикату | `FindIndexOf[T]([]T, func(T) bool) (T, int, bool)` | `item, index, ok := lo.FindIndexOf(slice, predicate)` |
| **FindLastIndexOf** | Находит последний элемент и индекс | `FindLastIndexOf[T]([]T, func(T) bool) (T, int, bool)` | `item, index, ok := lo.FindLastIndexOf(slice, predicate)` |
| **FindOrElse** | Находит элемент или возвращает значение по умолчанию | `FindOrElse[T]([]T, T, func(T) bool) T` | `item := lo.FindOrElse(slice, defaultValue, predicate)` |
| **FindKey** | Находит ключ по значению в карте | `FindKey[K, V](map[K]V, V) (K, bool)` | `key, ok := lo.FindKey(map[string]int{"a":1}, 1)` |
| **FindKeyBy** | Находит ключ по предикату | `FindKeyBy[K, V](map[K]V, func(K, V) bool) (K, bool)` | `key, ok := lo.FindKeyBy(myMap, func(k string, v int) bool { return v > 5 })` |
| **FindUniques** | Находит уникальные элементы | `FindUniques[T]([]T) []T` | `lo.FindUniques([]int{1,2,2,3})` → `[]int{1,3}` |
| **FindUniquesBy** | Находит уникальные по критерию | `FindUniquesBy[T, K]([]T, func(T) K) []T` | `lo.FindUniquesBy(numbers, func(i int) int { return i%3 })` |
| **FindDuplicates** | Находит дублирующиеся элементы | `FindDuplicates[T]([]T) []T` | `lo.FindDuplicates([]int{1,2,2,3})` → `[]int{2}` |
| **FindDuplicatesBy** | Находит дубликаты по критерию | `FindDuplicatesBy[T, K]([]T, func(T) K) []T` | `lo.FindDuplicatesBy(numbers, func(i int) int { return i%3 })` |
| **Min** | Находит минимальное значение | `Min[T]([]T) T` | `lo.Min([]int{3,1,4,1,5})` → `1` |
| **MinIndex** | Находит минимальное значение и индекс | `MinIndex[T]([]T) (T, int)` | `value, index := lo.MinIndex(slice)` |
| **MinBy** | Находит минимальное по критерию | `MinBy[T]([]T, func(T, T) bool) T` | `lo.MinBy(strings, func(a, b string) bool { return len(a) < len(b) })` |
| **MinIndexBy** | Находит минимальное и индекс по критерию | `MinIndexBy[T]([]T, func(T, T) bool) (T, int)` | `value, index := lo.MinIndexBy(slice, comparator)` |
| **Max** | Находит максимальное значение | `Max[T]([]T) T` | `lo.Max([]int{3,1,4,1,5})` → `5` |
| **MaxIndex** | Находит максимальное значение и индекс | `MaxIndex[T]([]T) (T, int)` | `value, index := lo.MaxIndex(slice)` |
| **MaxBy** | Находит максимальное по критерию | `MaxBy[T]([]T, func(T, T) bool) T` | `lo.MaxBy(strings, func(a, b string) bool { return len(a) > len(b) })` |
| **MaxIndexBy** | Находит максимальное и индекс по критерию | `MaxIndexBy[T]([]T, func(T, T) bool) (T, int)` | `value, index := lo.MaxIndexBy(slice, comparator)` |
| **Earliest** | Находит самое раннее время | `Earliest(...time.Time) time.Time` | `lo.Earliest(time1, time2, time3)` |
| **EarliestBy** | Находит самое раннее время по критерию | `EarliestBy[T]([]T, func(T) time.Time) T` | `lo.EarliestBy(events, func(e Event) time.Time { return e.CreatedAt })` |
| **Latest** | Находит самое позднее время | `Latest(...time.Time) time.Time` | `lo.Latest(time1, time2, time3)` |
| **LatestBy** | Находит самое позднее время по критерию | `LatestBy[T]([]T, func(T) time.Time) T` | `lo.LatestBy(events, func(e Event) time.Time { return e.CreatedAt })` |
| **First** | Возвращает первый элемент | `First[T]([]T) (T, bool)` | `first, ok := lo.First([]int{1,2,3})` |
| **FirstOrEmpty** | Возвращает первый элемент или ноль | `FirstOrEmpty[T]([]T) T` | `lo.FirstOrEmpty([]int{1,2,3})` → `1` |
| **FirstOr** | Возвращает первый элемент или значение по умолчанию | `FirstOr[T]([]T, T) T` | `lo.FirstOr([]int{}, 42)` → `42` |
| **Last** | Возвращает последний элемент | `Last[T]([]T) (T, bool)` | `last, ok := lo.Last([]int{1,2,3})` |
| **LastOrEmpty** | Возвращает последний элемент или ноль | `LastOrEmpty[T]([]T) T` | `lo.LastOrEmpty([]int{1,2,3})` → `3` |
| **LastOr** | Возвращает последний элемент или значение по умолчанию | `LastOr[T]([]T, T) T` | `lo.LastOr([]int{}, 42)` → `42` |
| **Nth** | Возвращает N-й элемент | `Nth[T]([]T, int) (T, error)` | `item, err := lo.Nth([]int{1,2,3}, 1)` |
| **NthOr** | Возвращает N-й элемент или значение по умолчанию | `NthOr[T]([]T, int, T) T` | `lo.NthOr([]int{1,2,3}, 5, 42)` → `42` |
| **NthOrEmpty** | Возвращает N-й элемент или ноль | `NthOrEmpty[T]([]T, int) T` | `lo.NthOrEmpty([]int{1,2,3}, 1)` → `2` |
| **Sample** | Возвращает случайный элемент | `Sample[T]([]T) T` | `lo.Sample([]string{"a","b","c"})` |
| **SampleBy** | Возвращает случайный элемент с генератором | `SampleBy[T]([]T, func(int) int) T` | `lo.SampleBy(slice, rand.Intn)` |
| **Samples** | Возвращает N случайных элементов | `Samples[T]([]T, int) []T` | `lo.Samples([]string{"a","b","c"}, 2)` |
| **SamplesBy** | Возвращает N случайных элементов с генератором | `SamplesBy[T]([]T, int, func(int) int) []T` | `lo.SamplesBy(slice, 3, rand.Intn)` |

---

## 10. Условные операции

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **Ternary** | Тернарный оператор | `Ternary[T](bool, T, T) T` | `result := lo.Ternary(age >= 18, "adult", "minor")` |
| **TernaryF** | Тернарный оператор с функциями | `TernaryF[T](bool, func() T, func() T) T` | `result := lo.TernaryF(condition, func() T { return expensive() }, func() T { return cheap() })` |
| **If** | Условная конструкция | `If[T](bool, T) IfElse[T]` | `result := lo.If(true, 1).ElseIf(false, 2).Else(3)` |
| **IfF** | Условная конструкция с функциями | `IfF[T](bool, func() T) IfElseF[T]` | `result := lo.IfF(condition, func() T { return value() }).Else(defaultValue)` |
| **Switch** | Switch-конструкция | `Switch[T, R](T) SwitchCase[T, R]` | `result := lo.Switch(value).Case(1, "one").Case(2, "two").Default("other")` |
| **SwitchF** | Switch-конструкция с функциями | `SwitchF[T, R](T) SwitchCaseF[T, R]` | `result := lo.Switch(value).CaseF(1, func() string { return "one" }).Default("other")` |

---

## 11. Манипуляции типов

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **IsNil** | Проверяет, что значение nil | `IsNil(any) bool` | `lo.IsNil((*int)(nil))` → `true` |
| **IsNotNil** | Проверяет, что значение не nil | `IsNotNil(any) bool` | `lo.IsNotNil(42)` → `true` |
| **ToPtr** | Создает указатель на значение | `ToPtr[T](T) *T` | `ptr := lo.ToPtr("hello")` |
| **Nil** | Возвращает nil указатель типа | `Nil[T]() *T` | `ptr := lo.Nil[int]()` |
| **EmptyableToPtr** | Создает указатель, если значение не пустое | `EmptyableToPtr[T](T) *T` | `ptr := lo.EmptyableToPtr("")` → `nil` |
| **FromPtr** | Извлекает значение из указателя | `FromPtr[T](*T) T` | `value := lo.FromPtr(&str)` |
| **FromPtrOr** | Извлекает значение или возвращает значение по умолчанию | `FromPtrOr[T](*T, T) T` | `value := lo.FromPtrOr(nil, "default")` |
| **ToSlicePtr** | Создает слайс указателей | `ToSlicePtr[T]([]T) []*T` | `ptrs := lo.ToSlicePtr([]string{"a","b"})` |
| **FromSlicePtr** | Извлекает значения из слайса указателей | `FromSlicePtr[T]([]*T) []T` | `values := lo.FromSlicePtr(ptrs)` |
| **FromSlicePtrOr** | Извлекает значения с fallback | `FromSlicePtrOr[T]([]*T, T) []T` | `values := lo.FromSlicePtrOr(ptrs, "default")` |
| **ToAnySlice** | Преобразует в слайс any | `ToAnySlice[T]([]T) []any` | `anys := lo.ToAnySlice([]int{1,2,3})` |
| **FromAnySlice** | Преобразует из слайса any | `FromAnySlice[T]([]any) ([]T, bool)` | `ints, ok := lo.FromAnySlice[int](anys)` |
| **Empty** | Возвращает нулевое значение типа | `Empty[T]() T` | `zero := lo.Empty[string]()` → `""` |
| **IsEmpty** | Проверяет, что значение пустое | `IsEmpty[T](T) bool` | `lo.IsEmpty("")` → `true` |
| **IsNotEmpty** | Проверяет, что значение не пустое | `IsNotEmpty[T](T) bool` | `lo.IsNotEmpty("hello")` → `true` |
| **Coalesce** | Возвращает первое непустое значение | `Coalesce[T](...T) (T, bool)` | `value, ok := lo.Coalesce(0, 1, 2)` → `1, true` |
| **CoalesceOrEmpty** | Возвращает первое непустое или пустое значение | `CoalesceOrEmpty[T](...T) T` | `value := lo.CoalesceOrEmpty(0, 1, 2)` → `1` |
| **CoalesceSlice** | Возвращает первый непустой слайс | `CoalesceSlice[T](...[]T) ([]T, bool)` | `slice, ok := lo.CoalesceSlice(nil, []int{1,2})` |
| **CoalesceSliceOrEmpty** | Возвращает первый непустой слайс или пустой | `CoalesceSliceOrEmpty[T](...[]T) []T` | `slice := lo.CoalesceSliceOrEmpty(nil, []int{1,2})` |
| **CoalesceMap** | Возвращает первую непустую карту | `CoalesceMap[K, V](...map[K]V) (map[K]V, bool)` | `m, ok := lo.CoalesceMap(nil, map[string]int{"a":1})` |
| **CoalesceMapOrEmpty** | Возвращает первую непустую карту или пустую | `CoalesceMapOrEmpty[K, V](...map[K]V) map[K]V` | `m := lo.CoalesceMapOrEmpty(nil, map[string]int{"a":1})` |

---

## 12. Функциональные помощники

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **Partial** | Частичное применение функции (1 аргумент) | `Partial[A, B, C](func(A, B) C, A) func(B) C` | `add5 := lo.Partial(add, 5)` |
| **Partial2** | Частичное применение функции (2 аргумента) | `Partial2[A, B, C, D](func(A, B, C) D, A) func(B, C) D` | `addWith42 := lo.Partial2(add3, 42)` |

---

## 13. Функции для работы с конкурентностью

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **Attempt** | Повторяет выполнение функции | `Attempt(int, func(int) error) (int, error)` | `iter, err := lo.Attempt(5, func(i int) error { return tryConnect() })` |
| **AttemptWithDelay** | Повторяет с задержкой | `AttemptWithDelay(int, time.Duration, func(int, time.Duration) error) (int, time.Duration, error)` | `iter, duration, err := lo.AttemptWithDelay(5, 1*time.Second, retryFunc)` |
| **AttemptWhile** | Повторяет пока условие истинно | `AttemptWhile(int, func(int) (error, bool)) (int, error)` | `count, err := lo.AttemptWhile(5, func(i int) (error, bool) { return err, shouldRetry })` |
| **AttemptWhileWithDelay** | Повторяет с задержкой пока условие истинно | `AttemptWhileWithDelay(int, time.Duration, func(int, time.Duration) (error, bool)) (int, time.Duration, error)` | `count, duration, err := lo.AttemptWhileWithDelay(5, 1*time.Second, retryFunc)` |
| **NewDebounce** | Создает debounce функцию | `NewDebounce(time.Duration, func()) (func(), func())` | `debounce, cancel := lo.NewDebounce(100*time.Millisecond, action)` |
| **NewDebounceBy** | Создает debounce по ключу | `NewDebounceBy[K](time.Duration, func(K, int)) (func(K), func(K))` | `debounce, cancel := lo.NewDebounceBy(100*time.Millisecond, keyedAction)` |
| **NewThrottle** | Создает throttle функцию | `NewThrottle(time.Duration, func()) (func(), func())` | `throttle, reset := lo.NewThrottle(100*time.Millisecond, action)` |
| **NewThrottleWithCount** | Создает throttle с лимитом вызовов | `NewThrottleWithCount(time.Duration, int, func()) (func(), func())` | `throttle, reset := lo.NewThrottleWithCount(100*time.Millisecond, 3, action)` |
| **NewThrottleBy** | Создает throttle по ключу | `NewThrottleBy[K](time.Duration, func(K)) (func(K), func(...K))` | `throttle, reset := lo.NewThrottleBy(100*time.Millisecond, keyedAction)` |
| **NewThrottleByWithCount** | Создает throttle по ключу с лимитом | `NewThrottleByWithCount[K](time.Duration, int, func(K)) (func(K), func(...K))` | `throttle, reset := lo.NewThrottleByWithCount(100*time.Millisecond, 3, keyedAction)` |
| **Synchronize** | Создает синхронизированную функцию | `Synchronize() Synchronizer` | `sync := lo.Synchronize(); sync.Do(func() { /* critical section */ })` |
| **Async** | Выполняет функцию асинхронно | `Async[T](func() T) <-chan T` | `ch := lo.Async(func() int { return heavyComputation() })` |
| **Async0-Async6** | Асинхронные версии для функций с разным количеством возвращаемых значений | `Async1[T](func() T) <-chan T` | `ch := lo.Async1(func() int { return 42 })` |
| **Transaction** | Реализует паттерн Saga | `NewTransaction[T]() Transaction[T]` | `tx := lo.NewTransaction[int]().Then(step1, rollback1).Then(step2, rollback2)` |
| **WaitFor** | Ожидает выполнения условия | `WaitFor(func(int) bool, time.Duration, time.Duration) (int, time.Duration, bool)` | `iter, duration, ok := lo.WaitFor(condition, 10*time.Second, 100*time.Millisecond)` |
| **WaitForWithContext** | Ожидает с контекстом | `WaitForWithContext(context.Context, func(context.Context, int) bool, time.Duration, time.Duration) (int, time.Duration, bool)` | `iter, duration, ok := lo.WaitForWithContext(ctx, condition, 10*time.Second, 100*time.Millisecond)` |

---

## 14. Обработка ошибок

| Функция | Описание | Сигнатура | Пример использования |
|---------|----------|-----------|---------------------|
| **Validate** | Создает ошибку при невыполнении условия | `Validate(bool, string, ...any) error` | `err := lo.Validate(len(slice) > 0, "slice must not be empty")` |
| **Must** | Паникует при ошибке, возвращает значение | `Must[T](T, error) T` | `value := lo.Must(strconv.Atoi("123"))` |
| **Must0-Must6** | Must для функций с разным количеством возвращаемых значений | `Must2[T, U](T, U, error) (T, U)` | `a, b := lo.Must2(parseTwo())` |
| **Try** | Безопасно выполняет функцию | `Try(func() error) bool` | `ok := lo.Try(func() error { return riskyOperation() })` |
| **Try1-Try6** | Try для функций с возвращаемыми значениями | `Try1[T](func() (T, error)) bool` | `ok := lo.Try1(func() (int, error) { return parse() })` |
| **TryOr** | Try с значением по умолчанию | `TryOr[T](func() (T, error), T) (T, bool)` | `value, ok := lo.TryOr(func() (string, error) { return parse() }, "default")` |
| **TryOr1-TryOr6** | TryOr для функций с разным количеством значений | `TryOr2[T, U](func() (T, U, error), T, U) (T, U, bool)` | `a, b, ok := lo.TryOr2(parseTwo, defaultA, defaultB)` |
| **TryWithErrorValue** | Try с возвратом значения panic | `TryWithErrorValue(func() error) (any, bool)` | `panicValue, ok := lo.TryWithErrorValue(func() error { panic("error") })` |
| **TryCatch** | Try с обработчиком ошибок | `TryCatch(func() error, func()) bool` | `ok := lo.TryCatch(riskyFunc, func() { cleanup() })` |
| **TryCatchWithErrorValue** | TryCatch с значением panic | `TryCatchWithErrorValue(func() error, func(any)) bool` | `ok := lo.TryCatchWithErrorValue(riskyFunc, func(val any) { log.Println(val) })` |
| **ErrorsAs** | Упрощенная версия errors.As | `ErrorsAs[T](error) (T, bool)` | `rateLimitErr, ok := lo.ErrorsAs[*RateLimitError](err)` |

---

## 15. Ограничения (Constraints)

| Тип | Описание |
|-----|----------|
| **Clonable** | Интерфейс для типов, которые могут быть клонированы |

```go
type Clonable[T any] interface {
    Clone() T
}
```

---

## Константы и переменные

| Константа | Значение | Описание |
|-----------|----------|----------|
| **LettersCharset** | `"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"` | Алфавитные символы |
| **NumbersCharset** | `"0123456789"` | Цифры |
| **AlphanumericCharset** | `LettersCharset + NumbersCharset` | Буквы и цифры |

---

## Стратегии распределения каналов

| Стратегия | Описание |
|-----------|----------|
| **DispatchingStrategyRoundRobin** | Циклическое распределение |
| **DispatchingStrategyRandom** | Случайное распределение |
| **DispatchingStrategyWeightedRandom** | Взвешенное случайное распределение |
| **DispatchingStrategyFirst** | Распределение в первый незаполненный канал |
| **DispatchingStrategyLeast** | Распределение в наименее заполненный канал |
| **DispatchingStrategyMost** | Распределение в наиболее заполненный канал |

---

## Производительность

Библиотека lo показывает отличную производительность:
- **lo.Map** в 7 раз быстрее чем go-funk (reflection-based)
- **lo.Map** имеет тот же профиль выделения памяти что и обычный `for` цикл  
- **lo.Map** всего на 4% медленнее чем `for` цикл
- **lop.Map** (параллельная версия) полезна для долго выполняющихся операций

---

## Параллельные версии

Многие функции имеют параллельные версии в пакете `lop`:
- `lop.Map` - параллельный Map
- `lop.Filter` - параллельный Filter  
- `lop.GroupBy` - параллельный GroupBy
- `lop.PartitionBy` - параллельный PartitionBy
- `lop.ForEach` - параллельный ForEach
- `lop.Times` - параллельный Times

```go
import lop "github.com/samber/lo/parallel"

result := lop.Map([]int{1,2,3,4}, func(x int, _ int) string {
    // выполняется в горутинах
    return heavyOperation(x)
})
```

---

## Мутирующие версии

Некоторые функции имеют мутирующие версии в пакете `lom`:
- `lom.Filter` - фильтрует на месте
- `lom.Map` - преобразует на месте
- `lom.Reverse` - переворачивает на месте
- `lom.Shuffle` - перемешивает на месте

```go
import lom "github.com/samber/lo/mutable"

slice := []int{1,2,3,4}
lom.Filter(&slice, func(x int) bool { return x%2 == 0 })
// slice теперь содержит [2,4]
```

Документация включает **15 основных категорий** функций:

## 🔥 Основные категории:

1. **Слайсы** (50+ функций) - Filter, Map, Reduce, GroupBy, Chunk, etc.
2. **Карты** (20+ функций) - Keys, Values, PickBy, OmitBy, Invert, etc.  
3. **Математика** - Range, Sum, Product, Mean, Clamp
4. **Строки** - PascalCase, CamelCase, RandomString, ChunkString
5. **Кортежи** - T2-T9, Zip, Unzip, CrossJoin
6. **Время** - Duration измерения
7. **Каналы** - ChannelDispatcher, Buffer, FanIn/FanOut
8. **Множества** - Contains, Intersect, Union, Difference
9. **Поиск** (30+ функций) - Find, IndexOf, Min/Max, Sample
10. **Условия** - Ternary, If/Else, Switch
11. **Типы** (25+ функций) - IsNil, ToPtr, FromPtr, Coalesce
12. **Функции** - Partial применение
13. **Конкурентность** - Attempt, Debounce, Throttle, Async
14. **Ошибки** - Must, Try, Validate, ErrorsAs
15. **Ограничения** - Clonable интерфейс

## 📊 Ключевые особенности:

- **Производительность**: lo.Map в 7 раз быстрее go-funk
- **Параллельность**: пакет `lop` для горутин  
- **Мутации**: пакет `lom` для изменений на месте
- **Типобезопасность**: полная поддержка дженериков Go 1.18+

Для каждой функции указаны **сигнатура, описание и примеры использования**. Это поможет быстро найти нужную функцию и понять, как её применять в коде.

Создам полные биндинги для библиотеки `lo` в goja:Я создал полные биндинги для библиотеки **lo** в goja, разделив их на **14 логических групп**:

## 🚀 Основные особенности:

### 📊 **Структурированность**
- **slice** - работа со слайсами (Filter, Map, Reduce, Chunk, etc.)
- **map** - работа с объектами (Keys, Values, PickBy, etc.)
- **math** - математические операции (Range, Sum, Mean, etc.)
- **string** - обработка строк (CamelCase, PascalCase, etc.)
- **search** - поиск и навигация (Find, Min/Max, Sample, etc.)
- **intersection** - работа с множествами (Contains, Union, etc.)
- **conditional** - условные операции (Ternary, etc.)
- **type** - манипуляции типов (IsNil, Coalesce, etc.)
- **error** - обработка ошибок (Try, TryCatch, etc.)
- **concurrency** - конкурентность (Attempt, WaitFor, etc.)
- **parallel** - параллельные версии функций
- **mutable** - мутирующие версии
- **constants** - константы библиотеки
- **util** - дополнительные утилиты

### 💡 **Адаптация для JavaScript**

1. **Типы**: Использую `interface{}` для универсальности
2. **Кортежи**: Преобразую в объекты с именованными полями
3. **Ошибки**: Возвращаю объекты с полями `error`, `ok`, `found`
4. **Контекст**: Поддерживаю Go context для сложных операций

## 📝 **Примеры использования в JS:**

```javascript
// Работа со слайсами
const evens = $lo.slice.filter([1,2,3,4,5,6], x => x % 2 === 0);
const doubled = $lo.slice.map([1,2,3], x => x * 2);
const sum = $lo.slice.reduce([1,2,3], (acc, x) => acc + x, 0);

// Строки
const camelCase = $lo.string.camelCase("hello_world"); // "helloWorld"
const chunks = $lo.string.chunkString("123456", 2); // ["12","34","56"]

// Поиск
const result = $lo.search.find([1,2,3,4], x => x > 2);
// {value: 3, found: true}

// Математика
const range = $lo.math.range(5); // [0,1,2,3,4]
const average = $lo.math.mean([1,2,3,4,5]); // 3

// Параллельная обработка
const parallel = $lo.parallel.map(bigArray, heavyOperation);

// Условия
const result = $lo.conditional.ternary(age >= 18, "adult", "minor");
```

## ⚡ **Ключевые преимущества:**

- ✅ **Полная функциональность** - все 150+ функций lo
- ✅ **JavaScript-friendly** - удобные возвращаемые значения
- ✅ **Типобезопасность** - корректная обработка типов
- ✅ **Производительность** - поддержка параллельных версий
- ✅ **Готовность к использованию** - полный рабочий пример

Теперь вся мощь библиотеки **lo** доступна в JavaScript через goja! 🎉


