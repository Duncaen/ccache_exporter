package ccache

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

type Counter int
const (
	None = 0
	CompilerProducedStdout = 1
	CompileFailed = 2
	InternalError = 3
	CacheMiss = 4
	PreprocessorError = 5
	CouldNotFindCompiler = 6
	MissingCacheFile = 7
	PreprocessedCacheHit = 8
	BadCompilerArguments = 9
	CalledForLink = 10
	FilesInCache = 11
	CacheSizeKibibyte = 12
	ObsoleteMaxFiles = 13
	ObsoleteMaxSize = 14
	UnsupportedSourceLanguage = 15
	BadOutputFile = 16
	NoInputFile = 17
	MultipleSourceFiles = 18
	AutoconfTest = 19
	UnsupportedCompilerOption = 20
	OutputToStdout = 21
	DirectCacheHit = 22
	CompilerProducedNoOutput = 23
	CompilerProducedEmptyOutput = 24
	ErrorHashingExtraFile = 25
	CompilerCheckFailed = 26
	CouldNotUsePrecompiledHeader = 27
	CalledForPreprocessing = 28
	CleanupsPerformed = 29
	UnsupportedCodeDirective = 30
	StatsZeroedTimestamp = 31
	CouldNotUseModules = 32
	DirectCacheMiss = 33
	PreprocessedCacheMiss = 34
	LocalStorageReadHit = 35
	LocalStorageReadMiss = 36
	RemoteStorageReadHit = 37
	RemoteStorageReadMiss = 38
	RemoteStorageError = 39
	RemoteStorageTimeout = 40
	Recache = 41
	UnsupportedEnvironmentVariable = 42
	LocalStorageWrite = 43
	LocalStorageHit = 44
	LocalStorageMiss = 45
	RemoteStorageWrite = 46
	RemoteStorageHit = 47
	RemoteStorageMiss = 48

	// 49-64: files in level 2 subdirs 0-f
	SubdirFilesBase = 49

	// 65-80: size (KiB) in level 2 subdirs 0-f
	SubdirSizeKibibyteBase = 65

	Disabled = 81
)

type Counters [82]uint64

func (counters *Counters) Read(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	i := 0
	for scanner.Scan() {
		n, err := strconv.ParseUint(scanner.Text(), 10, 64)
		if err != nil {
			return err
		}
		counters[i] += n
		i++
		if i == len(counters) {
			break
		}
	}
	return nil
}

func (counters *Counters) ReadAll(ccachedir string) error {
	for level1 := 0; level1 <= 0xF; level1++ {
		path := fmt.Sprintf("%s/%x/stats", ccachedir, level1)
		if err := counters.Read(path); err != nil {
			return err
		}
		for level2 := 0; level2 <= 0xF; level2++ {
			path := fmt.Sprintf("%s/%x/%x/stats", ccachedir, level1, level2)
			if err := counters.Read(path); err != nil {
				return err
			}
		}
	}
	return nil
}
