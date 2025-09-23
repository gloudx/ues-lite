package js

import (
	"time"

	"github.com/dop251/goja"
	"github.com/duke-git/lancet/v2/algorithm"
	"github.com/duke-git/lancet/v2/compare"
	"github.com/duke-git/lancet/v2/concurrency"
	"github.com/duke-git/lancet/v2/condition"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/cryptor"
	"github.com/duke-git/lancet/v2/datetime"
	"github.com/duke-git/lancet/v2/fileutil"
	"github.com/duke-git/lancet/v2/formatter"
	"github.com/duke-git/lancet/v2/function"
	"github.com/duke-git/lancet/v2/maputil"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/netutil"
	"github.com/duke-git/lancet/v2/pointer"
	"github.com/duke-git/lancet/v2/random"
	"github.com/duke-git/lancet/v2/retry"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/stream"
	"github.com/duke-git/lancet/v2/structs"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/duke-git/lancet/v2/system"
	"github.com/duke-git/lancet/v2/validator"
	"github.com/duke-git/lancet/v2/xerror"
)

// InitLancetBindings инициализирует все биндинги Lancet для goja
func InitLancetBindings(vm *goja.Runtime) {
	algorithmBinds(vm)
	sliceBinds(vm)
	strutilBinds(vm)
	mathutilBinds(vm)
	datetimeBinds(vm)
	cryptorBinds(vm)
	fileutilBinds(vm)
	netutilBinds(vm)
	concurrencyBinds(vm)
	validatorBinds(vm)
	convertorBinds(vm)
	maputilBinds(vm)
	randomBinds(vm)
	functionBinds(vm)
	xerrorBinds(vm)
	pointerBinds(vm)
	retryBinds(vm)
	conditionBinds(vm)
	compareBinds(vm)
	formatterBinds(vm)
	systemBinds(vm)
	structsBinds(vm)
	// tupleBinds(vm)
	streamBinds(vm)
}

// 1. Algorithm package bindings
func algorithmBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$algorithm", obj)

	// Sorting algorithms
	obj.Set("bubbleSort", algorithm.BubbleSort[int])
	obj.Set("countSort", algorithm.CountSort[int])
	obj.Set("heapSort", algorithm.HeapSort[int])
	obj.Set("insertionSort", algorithm.InsertionSort[int])
	obj.Set("mergeSort", algorithm.MergeSort[int])
	obj.Set("quickSort", algorithm.QuickSort[int])
	obj.Set("selectionSort", algorithm.SelectionSort[int])
	obj.Set("shellSort", algorithm.ShellSort[int])

	// Search algorithms
	obj.Set("binarySearch", algorithm.BinarySearch[int])
	obj.Set("binaryIterativeSearch", algorithm.BinaryIterativeSearch[int])
	obj.Set("linearSearch", algorithm.LinearSearch[int])

	// Cache
	obj.Set("newLRUCache", func(capacity int) any {
		return algorithm.NewLRUCache[string, any](capacity)
	})
}

// 2. Slice package bindings
func sliceBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$slice", obj)

	// Basic operations
	obj.Set("appendIfAbsent", slice.AppendIfAbsent[any])
	obj.Set("contain", slice.Contain[any])
	obj.Set("containBy", slice.ContainBy[any])
	obj.Set("containSubSlice", slice.ContainSubSlice[any])
	obj.Set("chunk", slice.Chunk[any])
	obj.Set("compact", slice.Compact[any])
	obj.Set("concat", slice.Concat[any])
	obj.Set("count", slice.Count[any])
	obj.Set("countBy", slice.CountBy[any])

	// Difference operations
	obj.Set("difference", slice.Difference[any])
	obj.Set("differenceBy", slice.DifferenceBy[any])
	obj.Set("differenceWith", slice.DifferenceWith[any])

	// Modification
	obj.Set("deleteAt", slice.DeleteAt[any])
	obj.Set("deleteRange", slice.DeleteRange[any])
	obj.Set("drop", slice.Drop[any])
	obj.Set("dropRight", slice.DropRight[any])
	obj.Set("dropWhile", slice.DropWhile[any])
	obj.Set("dropRightWhile", slice.DropRightWhile[any])

	// Comparison
	obj.Set("equal", slice.Equal[any])
	obj.Set("equalWith", slice.EqualWith[any, any])
	obj.Set("equalUnordered", slice.EqualUnordered[any])

	// Functional operations
	obj.Set("every", slice.Every[any])
	obj.Set("filter", slice.Filter[any])
	obj.Set("filterMap", slice.FilterMap[any, any])
	obj.Set("findBy", slice.FindBy[any])
	obj.Set("findLastBy", slice.FindLastBy[any])

	// Flattening
	obj.Set("flatten", slice.Flatten)
	obj.Set("flattenDeep", slice.FlattenDeep)
	obj.Set("flatMap", slice.FlatMap[any, any])

	// Iteration
	obj.Set("forEach", slice.ForEach[any])
	obj.Set("forEachWithBreak", slice.ForEachWithBreak[any])
	obj.Set("forEachConcurrent", slice.ForEachConcurrent[any])

	// Grouping
	obj.Set("groupBy", slice.GroupBy[any])
	obj.Set("groupWith", slice.GroupWith[any, any])

	// Set operations
	obj.Set("intersection", slice.Intersection[any])
	obj.Set("insertAt", slice.InsertAt[any])
	obj.Set("indexOf", slice.IndexOf[any])
	obj.Set("lastIndexOf", slice.LastIndexOf[any])

	// Transformation
	obj.Set("map", slice.Map[any, any])
	obj.Set("mapConcurrent", slice.MapConcurrent[any, any])
	obj.Set("merge", slice.Merge[any])
	obj.Set("reverse", slice.Reverse[any])

	// Reduction
	obj.Set("reduceBy", slice.ReduceBy[any, any])
	obj.Set("reduceRight", slice.ReduceRight[any, any])
	obj.Set("reduceConcurrent", slice.ReduceConcurrent[any])

	// Replacement
	obj.Set("replace", slice.Replace[any])
	obj.Set("replaceAll", slice.ReplaceAll[any])
	obj.Set("repeat", slice.Repeat[any])

	// Randomization
	obj.Set("shuffle", slice.Shuffle[any])
	obj.Set("shuffleCopy", slice.ShuffleCopy[any])

	// Sorting
	obj.Set("sort", slice.Sort[int])
	obj.Set("sortBy", slice.SortBy[any])
	obj.Set("some", slice.Some[any])

	// Set operations
	obj.Set("symmetricDifference", slice.SymmetricDifference[any])
	obj.Set("toSlice", slice.ToSlice[any])
	obj.Set("toSlicePointer", slice.ToSlicePointer[any])

	// Uniqueness
	obj.Set("unique", slice.Unique[any])
	obj.Set("uniqueBy", slice.UniqueBy[any, any])
	obj.Set("union", slice.Union[any])
	obj.Set("unionBy", slice.UnionBy[any, any])

	// Update and without
	obj.Set("updateAt", slice.UpdateAt[any])
	obj.Set("without", slice.Without[any])

	// Additional operations
	obj.Set("keyBy", slice.KeyBy[any, any])
	obj.Set("join", slice.Join[any])
	obj.Set("partition", slice.Partition[any])
}

// 3. String utilities bindings
func strutilBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$strutil", obj)

	// Substring operations
	obj.Set("after", strutil.After)
	obj.Set("afterLast", strutil.AfterLast)
	obj.Set("before", strutil.Before)
	obj.Set("beforeLast", strutil.BeforeLast)

	// Case conversion
	obj.Set("camelCase", strutil.CamelCase)
	obj.Set("capitalize", strutil.Capitalize)
	obj.Set("kebabCase", strutil.KebabCase)
	obj.Set("upperKebabCase", strutil.UpperKebabCase)
	obj.Set("lowerFirst", strutil.LowerFirst)
	obj.Set("upperFirst", strutil.UpperFirst)
	obj.Set("snakeCase", strutil.SnakeCase)
	obj.Set("upperSnakeCase", strutil.UpperSnakeCase)

	// Content checks
	obj.Set("containsAll", strutil.ContainsAll)
	obj.Set("containsAny", strutil.ContainsAny)
	obj.Set("isString", strutil.IsString)

	// Padding
	obj.Set("pad", strutil.Pad)
	obj.Set("padEnd", strutil.PadEnd)
	obj.Set("padStart", strutil.PadStart)

	// Manipulation
	obj.Set("reverse", strutil.Reverse)
	obj.Set("splitEx", strutil.SplitEx)
	obj.Set("substring", strutil.Substring)
	obj.Set("wrap", strutil.Wrap)
	obj.Set("unwrap", strutil.Unwrap)

	// Word operations
	obj.Set("splitWords", strutil.SplitWords)
	obj.Set("wordCount", strutil.WordCount)

	// Cleaning
	obj.Set("removeNonPrintable", strutil.RemoveNonPrintable)
	obj.Set("removeWhiteSpace", strutil.RemoveWhiteSpace)

	// Conversion
	obj.Set("stringToBytes", strutil.StringToBytes)
	obj.Set("bytesToString", strutil.BytesToString)

	// Validation
	obj.Set("isBlank", strutil.IsBlank)
	obj.Set("isNotBlank", strutil.IsNotBlank)
	obj.Set("hasPrefixAny", strutil.HasPrefixAny)
	obj.Set("hasSuffixAny", strutil.HasSuffixAny)

	// Search and replace
	obj.Set("indexOffset", strutil.IndexOffset)
	obj.Set("replaceWithMap", strutil.ReplaceWithMap)

	// Trimming
	obj.Set("trim", strutil.Trim)
	obj.Set("splitAndTrim", strutil.SplitAndTrim)

	// Security
	obj.Set("hideString", strutil.HideString)

	// Positioning
	obj.Set("subInBetween", strutil.SubInBetween)

	// Distance
	obj.Set("hammingDistance", strutil.HammingDistance)

	// Concatenation
	obj.Set("concat", strutil.Concat)
	obj.Set("ellipsis", strutil.Ellipsis)
}

// 4. Math utilities bindings
func mathutilBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$mathutil", obj)

	// Statistical functions
	obj.Set("average", mathutil.Average[float64])
	obj.Set("max", mathutil.Max[float64])
	obj.Set("maxBy", mathutil.MaxBy[any])
	obj.Set("min", mathutil.Min[float64])
	obj.Set("minBy", mathutil.MinBy[any])
	obj.Set("sum", mathutil.Sum[float64])

	// Number operations
	obj.Set("exponent", mathutil.Exponent)
	obj.Set("fibonacci", mathutil.Fibonacci)
	obj.Set("factorial", mathutil.Factorial)
	obj.Set("abs", mathutil.Abs[float64])
	obj.Set("div", mathutil.Div[float64])

	// Percentage and rounding
	obj.Set("percent", mathutil.Percent)
	obj.Set("roundToFloat", mathutil.RoundToFloat[float64])
	obj.Set("roundToString", mathutil.RoundToString[float64])
	obj.Set("truncRound", mathutil.TruncRound[float64])
	obj.Set("ceilToFloat", mathutil.CeilToFloat[float64])
	obj.Set("ceilToString", mathutil.CeilToString[float64])
	obj.Set("floorToFloat", mathutil.FloorToFloat[float64])
	obj.Set("floorToString", mathutil.FloorToString[float64])

	// Range functions
	obj.Set("range", mathutil.Range[int])
	obj.Set("rangeWithStep", mathutil.RangeWithStep[int])

	// Trigonometry
	obj.Set("angleToRadian", mathutil.AngleToRadian)
	obj.Set("radianToAngle", mathutil.RadianToAngle)
	obj.Set("cos", mathutil.Cos)
	obj.Set("sin", mathutil.Sin)

	// Geometry
	obj.Set("pointDistance", mathutil.PointDistance)

	// Number theory
	obj.Set("isPrime", mathutil.IsPrime)
	obj.Set("gcd", mathutil.GCD[int])
	obj.Set("lcm", mathutil.LCM[int])

	// Logarithm
	obj.Set("log", mathutil.Log)

	// Statistics
	obj.Set("variance", mathutil.Variance[float64])
	obj.Set("stdDev", mathutil.StdDev[float64])

	// Combinatorics
	obj.Set("permutation", mathutil.Permutation)
	obj.Set("combination", mathutil.Combination)
}

// 5. Datetime utilities bindings
func datetimeBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$datetime", obj)

	// Addition operations
	obj.Set("addDay", datetime.AddDay)
	obj.Set("addHour", datetime.AddHour)
	obj.Set("addMinute", datetime.AddMinute)
	obj.Set("addWeek", datetime.AddWeek)
	obj.Set("addMonth", datetime.AddMonth)
	obj.Set("addYear", datetime.AddYear)
	obj.Set("addDaySafe", datetime.AddDaySafe)
	obj.Set("addMonthSafe", datetime.AddMonthSafe)
	obj.Set("addYearSafe", datetime.AddYearSafe)

	// Begin/End operations
	obj.Set("beginOfMinute", datetime.BeginOfMinute)
	obj.Set("beginOfHour", datetime.BeginOfHour)
	obj.Set("beginOfDay", datetime.BeginOfDay)
	obj.Set("beginOfWeek", datetime.BeginOfWeek)
	obj.Set("beginOfMonth", datetime.BeginOfMonth)
	obj.Set("beginOfYear", datetime.BeginOfYear)
	obj.Set("endOfMinute", datetime.EndOfMinute)
	obj.Set("endOfHour", datetime.EndOfHour)
	obj.Set("endOfDay", datetime.EndOfDay)
	obj.Set("endOfWeek", datetime.EndOfWeek)
	obj.Set("endOfMonth", datetime.EndOfMonth)
	obj.Set("endOfYear", datetime.EndOfYear)

	// Current time functions
	obj.Set("getNowDate", datetime.GetNowDate)
	obj.Set("getNowTime", datetime.GetNowTime)
	obj.Set("getNowDateTime", datetime.GetNowDateTime)
	obj.Set("getTodayStartTime", datetime.GetTodayStartTime)
	obj.Set("getTodayEndTime", datetime.GetTodayEndTime)
	obj.Set("getZeroHourTimestamp", datetime.GetZeroHourTimestamp)
	obj.Set("getNightTimestamp", datetime.GetNightTimestamp)

	// Formatting
	obj.Set("formatTimeToStr", datetime.FormatTimeToStr)
	obj.Set("formatStrToTime", datetime.FormatStrToTime)

	// Unix time
	obj.Set("newUnix", datetime.NewUnix)
	obj.Set("newUnixNow", datetime.NewUnixNow)
	obj.Set("newFormat", datetime.NewFormat)
	obj.Set("newISO8601", datetime.NewISO8601)

	//TODO: Add more datetime functions as needed
	// obj.Set("toUnix", datetime.ToUnix)
	// obj.Set("toFormat", datetime.ToFormat)
	// obj.Set("toFormatForTpl", datetime.ToFormatForTpl)
	// obj.Set("toIso8601", datetime.ToIso8601)

	// Checks and calculations
	obj.Set("isLeapYear", datetime.IsLeapYear)
	obj.Set("betweenSeconds", datetime.BetweenSeconds)
	obj.Set("dayOfYear", datetime.DayOfYear)
	obj.Set("isWeekend", datetime.IsWeekend)
	obj.Set("daysBetween", datetime.DaysBetween)

	// Timestamps
	obj.Set("timestamp", datetime.Timestamp)
	obj.Set("timestampMilli", datetime.TimestampMilli)
	obj.Set("timestampMicro", datetime.TimestampMicro)
	obj.Set("timestampNano", datetime.TimestampNano)

	// Utilities
	obj.Set("trackFuncTime", datetime.TrackFuncTime)
	obj.Set("nowDateOrTime", datetime.NowDateOrTime)
}

// 6. Cryptography bindings
func cryptorBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$cryptor", obj)

	// AES encryption
	obj.Set("aesEcbEncrypt", cryptor.AesEcbEncrypt)
	obj.Set("aesEcbDecrypt", cryptor.AesEcbDecrypt)
	obj.Set("aesCbcEncrypt", cryptor.AesCbcEncrypt)
	obj.Set("aesCbcDecrypt", cryptor.AesCbcDecrypt)
	obj.Set("aesCtrCrypt", cryptor.AesCtrCrypt)
	obj.Set("aesCfbEncrypt", cryptor.AesCfbEncrypt)
	obj.Set("aesCfbDecrypt", cryptor.AesCfbDecrypt)
	obj.Set("aesOfbEncrypt", cryptor.AesOfbEncrypt)
	obj.Set("aesOfbDecrypt", cryptor.AesOfbDecrypt)
	obj.Set("aesGcmEncrypt", cryptor.AesGcmEncrypt)
	obj.Set("aesGcmDecrypt", cryptor.AesGcmDecrypt)

	// Base64
	obj.Set("base64StdEncode", cryptor.Base64StdEncode)
	obj.Set("base64StdDecode", cryptor.Base64StdDecode)

	// DES encryption
	obj.Set("desEcbEncrypt", cryptor.DesEcbEncrypt)
	obj.Set("desEcbDecrypt", cryptor.DesEcbDecrypt)
	obj.Set("desCbcEncrypt", cryptor.DesCbcEncrypt)
	obj.Set("desCbcDecrypt", cryptor.DesCbcDecrypt)
	obj.Set("desCtrCrypt", cryptor.DesCtrCrypt)
	obj.Set("desCfbEncrypt", cryptor.DesCfbEncrypt)
	obj.Set("desCfbDecrypt", cryptor.DesCfbDecrypt)
	obj.Set("desOfbEncrypt", cryptor.DesOfbEncrypt)
	obj.Set("desOfbDecrypt", cryptor.DesOfbDecrypt)

	// HMAC
	obj.Set("hmacMd5", cryptor.HmacMd5)
	obj.Set("hmacMd5WithBase64", cryptor.HmacMd5WithBase64)
	obj.Set("hmacSha1", cryptor.HmacSha1)
	obj.Set("hmacSha1WithBase64", cryptor.HmacSha1WithBase64)
	obj.Set("hmacSha256", cryptor.HmacSha256)
	obj.Set("hmacSha256WithBase64", cryptor.HmacSha256WithBase64)
	obj.Set("hmacSha512", cryptor.HmacSha512)
	obj.Set("hmacSha512WithBase64", cryptor.HmacSha512WithBase64)

	// MD5
	obj.Set("md5Byte", cryptor.Md5Byte)
	obj.Set("md5ByteWithBase64", cryptor.Md5ByteWithBase64)
	obj.Set("md5String", cryptor.Md5String)
	obj.Set("md5StringWithBase64", cryptor.Md5StringWithBase64)
	obj.Set("md5File", cryptor.Md5File)

	// SHA
	obj.Set("sha1", cryptor.Sha1)
	obj.Set("sha1WithBase64", cryptor.Sha1WithBase64)
	obj.Set("sha256", cryptor.Sha256)
	obj.Set("sha256WithBase64", cryptor.Sha256WithBase64)
	obj.Set("sha512", cryptor.Sha512)
	obj.Set("sha512WithBase64", cryptor.Sha512WithBase64)

	// RSA
	obj.Set("generateRsaKey", cryptor.GenerateRsaKey)
	obj.Set("rsaEncrypt", cryptor.RsaEncrypt)
	obj.Set("rsaDecrypt", cryptor.RsaDecrypt)
	obj.Set("generateRsaKeyPair", cryptor.GenerateRsaKeyPair)
	obj.Set("rsaEncryptOAEP", cryptor.RsaEncryptOAEP)
	obj.Set("rsaDecryptOAEP", cryptor.RsaDecryptOAEP)
	obj.Set("rsaSign", cryptor.RsaSign)
	obj.Set("rsaVerifySign", cryptor.RsaVerifySign)
}

// 7. File utilities bindings
func fileutilBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$fileutil", obj)

	// File operations
	obj.Set("clearFile", fileutil.ClearFile)
	obj.Set("createFile", fileutil.CreateFile)
	obj.Set("createDir", fileutil.CreateDir)
	obj.Set("copyFile", fileutil.CopyFile)
	obj.Set("copyDir", fileutil.CopyDir)
	obj.Set("removeFile", fileutil.RemoveFile)
	obj.Set("removeDir", fileutil.RemoveDir)

	// File info
	obj.Set("fileMode", fileutil.FileMode)
	obj.Set("miMeType", fileutil.MiMeType)
	obj.Set("isExist", fileutil.IsExist)
	obj.Set("isLink", fileutil.IsLink)
	obj.Set("isDir", fileutil.IsDir)
	obj.Set("listFileNames", fileutil.ListFileNames)
	obj.Set("currentPath", fileutil.CurrentPath)
	obj.Set("fileSize", fileutil.FileSize)
	obj.Set("mTime", fileutil.MTime)
	obj.Set("sha", fileutil.Sha)

	// Reading
	obj.Set("readFileToString", fileutil.ReadFileToString)
	obj.Set("readFileByLine", fileutil.ReadFileByLine)
	obj.Set("readFile", fileutil.ReadFile)
	obj.Set("chunkRead", fileutil.ChunkRead)
	obj.Set("parallelChunkRead", fileutil.ParallelChunkRead)

	// Writing
	obj.Set("writeBytesToFile", fileutil.WriteBytesToFile)
	obj.Set("writeStringToFile", fileutil.WriteStringToFile)

	// CSV operations
	obj.Set("readCsvFile", fileutil.ReadCsvFile)
	obj.Set("writeCsvFile", fileutil.WriteCsvFile)
	obj.Set("writeMapsToCsv", fileutil.WriteMapsToCsv)

	// Compression
	obj.Set("zip", fileutil.Zip)
	obj.Set("zipAppendEntry", fileutil.ZipAppendEntry)
	obj.Set("unZip", fileutil.UnZip)
	obj.Set("isZipFile", fileutil.IsZipFile)
}

// 8. Network utilities bindings
func netutilBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$netutil", obj)

	// URL operations
	obj.Set("convertMapToQueryString", netutil.ConvertMapToQueryString)
	obj.Set("encodeUrl", netutil.EncodeUrl)
	obj.Set("buildUrl", netutil.BuildUrl)
	obj.Set("addQueryParams", netutil.AddQueryParams)

	// IP operations
	obj.Set("getInternalIp", netutil.GetInternalIp)
	obj.Set("getIps", netutil.GetIps)
	obj.Set("getMacAddrs", netutil.GetMacAddrs)
	obj.Set("getPublicIpInfo", netutil.GetPublicIpInfo)
	obj.Set("getRequestPublicIp", netutil.GetRequestPublicIp)
	obj.Set("isPublicIP", netutil.IsPublicIP)
	obj.Set("isInternalIP", netutil.IsInternalIP)

	// HTTP operations
	obj.Set("httpRequest", netutil.HttpRequest{})
	obj.Set("newHttpClient", netutil.NewHttpClient)

	//TODO: Add more HTTP operations as needed
	// obj.Set("sendRequest", netutil.SendRequest)
	// obj.Set("decodeResponse", netutil.DecodeResponse)

	obj.Set("structToUrlValues", netutil.StructToUrlValues)

	// File transfer
	obj.Set("downloadFile", netutil.DownloadFile)
	obj.Set("uploadFile", netutil.UploadFile)

	// Connection testing
	obj.Set("isPingConnected", netutil.IsPingConnected)
	obj.Set("isTelnetConnected", netutil.IsTelnetConnected)
}

// 9. Concurrency bindings
func concurrencyBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$concurrency", obj)

	// Channel operations
	obj.Set("newChannel", func() any {
		return concurrency.NewChannel[any]()
	})

	// TODO: Add more channel operations as needed
	// obj.Set("bridge", concurrency.Bridge[any])
	// obj.Set("fanIn", concurrency.FanIn[any])
	// obj.Set("generate", concurrency.Generate[any])
	// obj.Set("or", concurrency.Or)
	// obj.Set("orDone", concurrency.OrDone[any])
	// obj.Set("repeat", concurrency.Repeat[any])
	// obj.Set("repeatFn", concurrency.RepeatFn[any])
	// obj.Set("take", concurrency.Take[any])
	// obj.Set("tee", concurrency.Tee[any])

	// Locking mechanisms
	obj.Set("newKeyedLocker", func(timeout time.Duration) any {
		return concurrency.NewKeyedLocker[string](timeout)
	})
	obj.Set("newRWKeyedLocker", func(timeout time.Duration) any {
		return concurrency.NewRWKeyedLocker[string](timeout)
	})
	obj.Set("newTryKeyedLocker", func() any {
		return concurrency.NewTryKeyedLocker[string]()
	})
}

// 10. Validator bindings
func validatorBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$validator", obj)

	// Character content validation
	obj.Set("containChinese", validator.ContainChinese)
	obj.Set("containLetter", validator.ContainLetter)
	obj.Set("containLower", validator.ContainLower)
	obj.Set("containUpper", validator.ContainUpper)

	// String type validation
	obj.Set("isAlpha", validator.IsAlpha)
	obj.Set("isAllUpper", validator.IsAllUpper)
	obj.Set("isAllLower", validator.IsAllLower)
	obj.Set("isAlphaNumeric", validator.IsAlphaNumeric)
	obj.Set("isASCII", validator.IsASCII)
	obj.Set("isPrintable", validator.IsPrintable)

	// Encoding validation
	obj.Set("isBase64", validator.IsBase64)
	obj.Set("isBase64URL", validator.IsBase64URL)
	obj.Set("isGBK", validator.IsGBK)
	obj.Set("isBin", validator.IsBin)
	obj.Set("isHex", validator.IsHex)

	// Chinese-specific validation
	obj.Set("isChineseMobile", validator.IsChineseMobile)
	obj.Set("isChineseIdNum", validator.IsChineseIdNum)
	obj.Set("isChinesePhone", validator.IsChinesePhone)

	// Financial validation
	obj.Set("isCreditCard", validator.IsCreditCard)
	obj.Set("isVisa", validator.IsVisa)
	obj.Set("isMasterCard", validator.IsMasterCard)
	obj.Set("isAmericanExpress", validator.IsAmericanExpress)
	obj.Set("isUnionPay", validator.IsUnionPay)
	obj.Set("isChinaUnionPay", validator.IsChinaUnionPay)

	// Network validation
	obj.Set("isDns", validator.IsDns)
	obj.Set("isEmail", validator.IsEmail)
	obj.Set("isIp", validator.IsIp)
	obj.Set("isIpV4", validator.IsIpV4)
	obj.Set("isIpV6", validator.IsIpV6)
	obj.Set("isIpPort", validator.IsIpPort)
	obj.Set("isUrl", validator.IsUrl)

	// Data type validation
	obj.Set("isEmptyString", validator.IsEmptyString)
	obj.Set("isFloat", validator.IsFloat)
	obj.Set("isFloatStr", validator.IsFloatStr)
	obj.Set("isNumber", validator.IsNumber)
	obj.Set("isNumberStr", validator.IsNumberStr)
	obj.Set("isInt", validator.IsInt)
	obj.Set("isIntStr", validator.IsIntStr)
	obj.Set("isJSON", validator.IsJSON)
	obj.Set("isJWT", validator.IsJWT)
	obj.Set("isZeroValue", validator.IsZeroValue)

	// Password validation
	obj.Set("isStrongPassword", validator.IsStrongPassword)
	obj.Set("isWeakPassword", validator.IsWeakPassword)

	// Pattern validation
	obj.Set("isRegexMatch", validator.IsRegexMatch)
}

// 11. Convertor bindings
func convertorBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$convertor", obj)

	// Color conversion
	obj.Set("colorHexToRGB", convertor.ColorHexToRGB)
	obj.Set("colorRGBToHex", convertor.ColorRGBToHex)

	// Type conversion
	obj.Set("toBool", convertor.ToBool)
	obj.Set("toBytes", convertor.ToBytes)
	obj.Set("toChar", convertor.ToChar)
	obj.Set("toChannel", convertor.ToChannel[any])
	obj.Set("toFloat", convertor.ToFloat)
	obj.Set("toInt", convertor.ToInt)
	obj.Set("toJson", convertor.ToJson)
	obj.Set("toMap", func(slice []any, mapper func(any) (any, any)) any {
		return convertor.ToMap[any, any](slice, mapper)
	})
	obj.Set("toPointer", convertor.ToPointer[any])
	obj.Set("toString", convertor.ToString)

	// Structure conversion
	obj.Set("structToMap", convertor.StructToMap)
	obj.Set("mapToSlice", convertor.MapToSlice[any, any, any])
	obj.Set("copyProperties", convertor.CopyProperties[any, any])
	obj.Set("deepClone", convertor.DeepClone[any])
	obj.Set("toInterface", convertor.ToInterface)

	// Encoding conversion
	obj.Set("encodeByte", convertor.EncodeByte)
	obj.Set("decodeByte", convertor.DecodeByte)
	obj.Set("utf8ToGbk", convertor.Utf8ToGbk)
	obj.Set("gbkToUtf8", convertor.GbkToUtf8)

	// Base64 conversion
	obj.Set("toStdBase64", convertor.ToStdBase64)
	obj.Set("toUrlBase64", convertor.ToUrlBase64)
	obj.Set("toRawStdBase64", convertor.ToRawStdBase64)
	obj.Set("toRawUrlBase64", convertor.ToRawUrlBase64)

	// Big number conversion
	obj.Set("toBigInt", convertor.ToBigInt[any])
}

// 12. Map utilities bindings
func maputilBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$maputil", obj)

	// Basic operations
	obj.Set("mapTo", maputil.MapTo)
	obj.Set("forEach", maputil.ForEach[any, any])
	obj.Set("hasKey", maputil.HasKey[any, any])
	obj.Set("getOrSet", maputil.GetOrSet[any, any])
	obj.Set("getOrDefault", maputil.GetOrDefault[any, any])

	// Filtering operations
	obj.Set("filter", maputil.Filter[any, any])
	obj.Set("filterByKeys", maputil.FilterByKeys[any, any])
	obj.Set("filterByValues", maputil.FilterByValues[any, any])
	obj.Set("omitBy", maputil.OmitBy[any, any])
	obj.Set("omitByKeys", maputil.OmitByKeys[any, any])
	obj.Set("omitByValues", maputil.OmitByValues[any, any])

	// Set operations
	obj.Set("intersect", maputil.Intersect[any, any])
	obj.Set("merge", maputil.Merge[any, any])
	obj.Set("minus", maputil.Minus[any, any])
	obj.Set("isDisjoint", maputil.IsDisjoint[any, any])

	// Key/Value extraction
	obj.Set("keys", maputil.Keys[any, any])
	obj.Set("keysBy", maputil.KeysBy[any, any, any])
	obj.Set("values", maputil.Values[any, any])
	obj.Set("valuesBy", maputil.ValuesBy[any, any, any])

	// Transformation
	obj.Set("mapKeys", maputil.MapKeys[any, any, any])
	obj.Set("mapValues", maputil.MapValues[any, any, any])
	obj.Set("transform", maputil.Transform[any, any, any, any])

	// Entry operations
	obj.Set("entries", maputil.Entries[any, any])
	obj.Set("fromEntries", maputil.FromEntries[any, any])

	// Structure conversion
	obj.Set("mapToStruct", maputil.MapToStruct)
	obj.Set("toSortedSlicesDefault", maputil.ToSortedSlicesDefault[string, any])
	obj.Set("toSortedSlicesWithComparator", maputil.ToSortedSlicesWithComparator[string, any])

	// Special map types
	obj.Set("newOrderedMap", func() any {
		return maputil.NewOrderedMap[any, any]()
	})
	obj.Set("newConcurrentMap", func(shardCount int) any {
		return maputil.NewConcurrentMap[any, any](shardCount)
	})

	// Sorting and searching
	obj.Set("sortByKey", maputil.SortByKey[string, any])
	obj.Set("findValuesBy", maputil.FindValuesBy[any, any])
}

// 13. Random utilities bindings
func randomBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$random", obj)

	// Basic random generation
	obj.Set("randBytes", random.RandBytes)
	obj.Set("randInt", random.RandInt)
	obj.Set("randString", random.RandString)
	obj.Set("randUpper", random.RandUpper)
	obj.Set("randLower", random.RandLower)
	obj.Set("randNumeral", random.RandNumeral)
	obj.Set("randNumeralOrLetter", random.RandNumeralOrLetter)
	obj.Set("randSymbolChar", random.RandSymbolChar)

	// UUID generation
	obj.Set("uuIdV4", random.UUIdV4)

	// Float generation
	obj.Set("randFloat", random.RandFloat)
	obj.Set("randFloats", random.RandFloats)

	// Boolean generation
	obj.Set("randBool", random.RandBool)
	obj.Set("randBoolSlice", random.RandBoolSlice)

	// Slice generation
	obj.Set("randUniqueIntSlice", random.RandUniqueIntSlice)
	obj.Set("randIntSlice", random.RandIntSlice)
	obj.Set("randStringSlice", random.RandStringSlice)
	obj.Set("randFromGivenSlice", random.RandFromGivenSlice[any])
	obj.Set("randSliceFromGivenSlice", random.RandSliceFromGivenSlice[any])

	// Number with length
	obj.Set("randNumberOfLength", random.RandNumberOfLength)
}

// 14. Function utilities bindings
func functionBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$function", obj)

	// Control flow
	obj.Set("after", function.After)
	obj.Set("before", function.Before)
	obj.Set("delay", function.Delay)
	obj.Set("debounce", function.Debounce)
	obj.Set("throttle", function.Throttle)
	obj.Set("schedule", function.Schedule)

	// TODO: Function composition
	// obj.Set("curryFn", function.CurryFn[any, any, any])
	// obj.Set("compose", function.Compose[any, any, any])
	// obj.Set("pipeline", function.Pipeline[any, any, any])

	// Conditional execution
	// obj.Set("acceptIf", function.AcceptIf[any, any])

	// Logical operations
	obj.Set("and", function.And[any])
	obj.Set("or", function.Or[any])
	obj.Set("negate", function.Negate[any])
	obj.Set("nor", function.Nor[any])
	obj.Set("nand", function.Nand[any])
	obj.Set("xnor", function.Xnor[any])

	// Utilities
	obj.Set("newWatcher", func() any {
		return function.NewWatcher()
	})
}

// 15. Error handling bindings
func xerrorBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$xerror", obj)

	// Error creation
	obj.Set("new", xerror.New)
	obj.Set("wrap", xerror.Wrap)
	obj.Set("unwrap", xerror.Unwrap)

	// Utilities
	obj.Set("tryUnwrap", xerror.TryUnwrap[any])

	// TODO: Error handling
	// obj.Set("tryCatch", xerror.TryCatch)
}

// 16. Pointer utilities bindings
func pointerBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$pointer", obj)

	obj.Set("extractPointer", pointer.ExtractPointer)
	obj.Set("of", pointer.Of[any])
	obj.Set("unwrap", pointer.Unwrap[any])
	obj.Set("unwrapOr", pointer.UnwrapOr[any])

	// TODO: UnwrapOrDefault is not defined in the original code, assuming it exists
	// obj.Set("unwrapOrDefault", pointer.UnwrapOrDefault[any])
}

// 17. Retry utilities bindings
func retryBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$retry", obj)

	obj.Set("retry", retry.Retry)
	obj.Set("context", retry.Context)
	obj.Set("retryTimes", retry.RetryTimes)
	obj.Set("retryWithCustomBackoff", retry.RetryWithCustomBackoff)
	obj.Set("retryWithLinearBackoff", retry.RetryWithLinearBackoff)
	obj.Set("retryWithExponentialWithJitterBackoff", retry.RetryWithExponentialWithJitterBackoff)
}

// 18. Condition utilities bindings
func conditionBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$condition", obj)

	obj.Set("bool", condition.Bool[any])
	obj.Set("and", condition.And[any, any])
	obj.Set("or", condition.Or[any, any])
	obj.Set("xor", condition.Xor[any, any])
	obj.Set("nor", condition.Nor[any, any])
	obj.Set("xnor", condition.Xnor[any, any])
	obj.Set("nand", condition.Nand[any, any])
	obj.Set("ternaryOperator", condition.TernaryOperator[any, any])
}

// 19. Compare utilities bindings
func compareBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$compare", obj)

	obj.Set("equal", compare.Equal)
	obj.Set("equalValue", compare.EqualValue)
	obj.Set("lessThan", compare.LessThan)
	obj.Set("greaterThan", compare.GreaterThan)
	obj.Set("lessOrEqual", compare.LessOrEqual)
	obj.Set("greaterOrEqual", compare.GreaterOrEqual)
	obj.Set("inDelta", compare.InDelta[float64])
}

// 20. Formatter utilities bindings
func formatterBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$formatter", obj)

	obj.Set("comma", formatter.Comma[float64])
	obj.Set("pretty", formatter.Pretty)
	obj.Set("prettyToWriter", formatter.PrettyToWriter)
	obj.Set("decimalBytes", formatter.DecimalBytes)
	obj.Set("binaryBytes", formatter.BinaryBytes)
	obj.Set("parseDecimalBytes", formatter.ParseDecimalBytes)
	obj.Set("parseBinaryBytes", formatter.ParseBinaryBytes)
}

// 21. System utilities bindings
func systemBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$system", obj)

	// OS detection
	obj.Set("isWindows", system.IsWindows)
	obj.Set("isLinux", system.IsLinux)
	obj.Set("isMac", system.IsMac)
	obj.Set("getOsBits", system.GetOsBits)

	// Environment variables
	obj.Set("getOsEnv", system.GetOsEnv)
	obj.Set("setOsEnv", system.SetOsEnv)
	obj.Set("removeOsEnv", system.RemoveOsEnv)
	obj.Set("compareOsEnv", system.CompareOsEnv)

	// Process management
	obj.Set("execCommand", system.ExecCommand)
	obj.Set("startProcess", system.StartProcess)
	obj.Set("stopProcess", system.StopProcess)
	obj.Set("killProcess", system.KillProcess)
	obj.Set("getProcessInfo", system.GetProcessInfo)
}

// 22. Structs utilities bindings
func structsBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$structs", obj)

	obj.Set("new", structs.New)
	obj.Set("toMap", func(s any) (map[string]any, error) {
		return structs.New(s).ToMap()
	})
	obj.Set("fields", func(s any) any {
		return structs.New(s).Fields()
	})
	obj.Set("isStruct", func(s any) bool {
		return structs.New(s).IsStruct()
	})
}

// TODO: 23. Tuple utilities bindings
// func tupleBinds(vm *goja.Runtime) {
// 	obj := vm.NewObject()
// 	vm.Set("$tuple", obj)

// 	// Tuple2
// 	obj.Set("tuple2", tuple.Tuple2[any, any])
// 	obj.Set("zip2", tuple.Zip2[any, any])
// 	obj.Set("unzip2", tuple.Unzip2[any, any])

// 	// Tuple3
// 	obj.Set("tuple3", tuple.Tuple3[any, any, any])
// 	obj.Set("zip3", tuple.Zip3[any, any, any])
// 	obj.Set("unzip3", tuple.Unzip3[any, any, any])

// 	// Tuple4
// 	obj.Set("tuple4", tuple.Tuple4[any, any, any, any])
// 	obj.Set("zip4", tuple.Zip4[any, any, any, any])
// 	obj.Set("unzip4", tuple.Unzip4[any, any, any, any])

// 	// Tuple5
// 	obj.Set("tuple5", tuple.Tuple5[any, any, any, any, any])
// 	obj.Set("zip5", tuple.Zip5[any, any, any, any, any])
// 	obj.Set("unzip5", tuple.Unzip5[any, any, any, any, any])
// }

// 24. Stream utilities bindings
func streamBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$stream", obj)

	// Stream creation
	obj.Set("of", stream.Of[any])
	obj.Set("fromSlice", stream.FromSlice[any])
	obj.Set("fromChannel", stream.FromChannel[any])
	obj.Set("fromRange", stream.FromRange[int])
	obj.Set("generate", stream.Generate[any])
	obj.Set("concat", stream.Concat[any])

	// Stream operations would be methods on stream objects
	// This would require more complex bindings to handle method chaining
}

// Usage example:
func main() {
	vm := goja.New()
	InitLancetBindings(vm)

	// Now you can use Lancet functions in JavaScript:
	_, err := vm.RunString(`
        // String operations
        let result = $strutil.camelCase("hello_world");
        console.log(result); // "helloWorld"
        
        // Array operations
        let arr = [1, 2, 3, 4, 5];
        let doubled = $slice.map(arr, x => x * 2);
        console.log(doubled); // [2, 4, 6, 8, 10]
        
        // Validation
        let isValid = $validator.isEmail("test@example.com");
        console.log(isValid); // true
        
        // Random generation
        let randomStr = $random.randString(10);
        console.log(randomStr); // random 10-char string
        
        // Math operations
        let avg = $mathutil.average(1, 2, 3, 4, 5);
        console.log(avg); // 3
        
        // Date operations
        let now = new Date();
        let tomorrow = $datetime.addDay(now, 1);
        console.log(tomorrow);
    `)

	if err != nil {
		panic(err)
	}
}
