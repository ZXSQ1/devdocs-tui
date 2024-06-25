package doc_manager

import (
	"os"
	"os/exec"
	"strings"

	"github.com/ZXSQ1/dopen/files"
	"github.com/ZXSQ1/dopen/utils"
)

const (
	rootDirName = "dopen"

	asyncExt = ".async"
	rawExt   = ".raw"
	indexExt = ".index"
)

var (
	rootDir     = utils.GetEnvironVar("HOME") + "/.cache/" + rootDirName
	tempDir     = rootDir + "/.temp"
	ColumnWidth = "80"
)

/*
description: gets the directory for the language
arguments:

	language: the string language specified

return: the directory for the language
*/
func GetLanguageDir(language string) string {
	return rootDir + "/" + language
}

/*
description: initializes the language directory
arguments:

	language: the language to intialize the directory of

return
*/
func Init(language string) {
	languageDir := GetLanguageDir(language)

	if !files.IsExists(rootDir) {
		os.MkdirAll(rootDir, 0744)
	}

	if !files.IsExists(tempDir) {
		os.Mkdir(tempDir, 0744)
	}

	if !files.IsExists(languageDir) {
		os.Mkdir(languageDir, 0744)
	}
}

func DownloadDocs(language string) {
	proc := exec.Command("dedoc", "download", language)
	proc.Stderr = os.Stderr
	
	if proc.Run() != nil {
		os.Exit(1)
	}
}

/*
description: fetches the raw docs unmodified docs
arguments:

	language: the language to fetch the raw docs of

return: the error
*/
func FetchRawDocs(language string) error {
	languageDir := GetLanguageDir(language)

	DownloadDocs(language)

	proc := exec.Command("dedoc", "search", language)
	proc.Stderr = os.Stderr
	proc.Stdin = os.Stdin

	out, err := proc.Output()

	if err != nil {
		os.Exit(1)
	}

	strOut := string(out)
	strOut = strings.Join(strings.Split(strOut, "\n")[2:], "\n")

	return files.WriteFile(languageDir+"/"+language+asyncExt+rawExt, []byte(strOut))
}

/*
description: filters the doc entry
arguments:

	entry: the unfiltered doc entry

return: a slice containing
 1. the code of the entry at index 0
 2. the name of the entry at index 1
*/
func FilterDocEntry(entry string) []string {
	entry = strings.TrimSpace(entry)
	entryParts := strings.Split(entry, " ")

	entryNumber := entryParts[0]
	entryName := entryParts[len(entryParts)-1]

	return []string{entryNumber, entryName}
}

/*
description: indexes the docs in a cache file
arguments:

	language: the language to indexed

return: an error
*/
func IndexDocs(language string) error {
	languageDir := GetLanguageDir(language)
	out, _ := files.ReadFile(languageDir + "/" + language + asyncExt + rawExt)
	raw := strings.TrimSpace(string(out))

	result := ""
	parentName := ""

	for _, entry := range strings.Split(raw, "\n") {
		entryParts := FilterDocEntry(entry)
		entryName := entryParts[1]

		if strings.HasPrefix(entryName, "#") {
			result = result + entryName + " "
		} else {
			parentName = entryName
			result = result + "\n" + parentName + " "
		}
	}

	result = strings.TrimSpace(result)

	return files.WriteFile(languageDir+"/"+language+asyncExt+indexExt, []byte(result))
}

/*
description: caches the fetched raw and indexed docs
arguments:

	language: language to cache docs for

return: an error
*/
func CacheDocs(language string) error {
	languageDir := GetLanguageDir(language)

	asyncRawPath := languageDir + "/" + language + asyncExt + rawExt
	rawPath := languageDir + "/" + language + rawExt
	asyncIndexPath := languageDir + "/" + language + asyncExt + indexExt
	indexPath := languageDir + "/" + language + indexExt

	err := os.Rename(asyncIndexPath, indexPath)

	if err != nil {
		return err
	}

	err = os.Rename(asyncRawPath, rawPath)

	if err != nil {
		return err
	}

	return nil
}

/*
description: searches for some doc
arguments:

	language: the language which contains the doc
	docEntryName: the doc string to search for the completed version of

return: the full doc entry
*/
func SearchDocs(language, docEntryName string) (fullDocEntryName string) {
	if !strings.HasPrefix(docEntryName, "#") {
		return docEntryName
	}

	languageDir := GetLanguageDir(language)

	indexOut, _ := files.ReadFile(languageDir + "/" + language + indexExt)
	index := string(indexOut)

	for _, line := range strings.Split(index, "\n") {
		if strings.Contains(line, docEntryName) {
			return strings.Split(line, " ")[0] + docEntryName
		}
	}

	return "doc not found"
}

/*
description: prepares the docs by fetching & indexing & caching
arguments:

	language: the language to prepare the docs for

return:
*/
func PrepareDocs(language string) {
	languageDir := GetLanguageDir(language)

	if !files.IsExists(languageDir + "/" + language + rawExt) {
		Init(language)
		FetchRawDocs(language)
		IndexDocs(language)
	}

	CacheDocs(language)

	go func() {
		FetchRawDocs(language)
		IndexDocs(language)
	}()
}

/*
description: opens the interface to open the documentation
arguments:

	language: the language to search for the docs in

return:
*/
func OpenDocs(language string) {
	languageDir := GetLanguageDir(language)

	PrepareDocs(language)

	messenger := &utils.Messenger{}
	out, _ := files.ReadFile(languageDir + "/" + language + rawExt)
	messenger.Write(out)

	// fzf

	fzfOptions := []string{"--layout=reverse"}

	if fzfDefaultOptions := utils.GetEnvironVar("FZF_DEFAULT_OPTS"); fzfDefaultOptions != "" {
		fzfOptions = strings.Split(fzfDefaultOptions, " ")
	}

	proc := exec.Command("fzf", fzfOptions...)
	proc.Stdin = messenger
	proc.Stdout = messenger
	proc.Stderr = os.Stderr

	proc.Run()

	// filter chosen doc

	docEntryName := FilterDocEntry(string(messenger.Message))[1]
	docEntryName = SearchDocs(language, docEntryName)

	messenger = &utils.Messenger{}
	messenger.Write([]byte(docEntryName))

	// dedoc open

	proc = exec.Command("dedoc", "open", language, string(messenger.Message))
	messenger = &utils.Messenger{}

	proc.Stdout = messenger
	proc.Stderr = os.Stderr

	err := proc.Run()

	if err != nil {
		os.Exit(1)
	}

	// glow -p

	tempFile := tempDir + "/doc"

	if files.IsExists(tempFile) {
		os.Remove(tempFile)
	}

	files.WriteFile(tempFile, messenger.Message)

	proc = exec.Command("glow", "-p", "-w", ColumnWidth, tempFile)
	proc.Stdin = os.Stderr
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr

	proc.Run()
}
