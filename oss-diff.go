package main

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type List struct {
	Name        string
	Prefix      string
	Marker      string
	MaxKeys     int
	Delimiter   string
	IsTruncated bool
	NextMarker  string
	Files       []File `xml:"Contents"`
}
type File struct {
	Name         string `xml:"Key"`
	LastModified string
	ETag         string
	Size         int64
}

var API string

func request(method, remotePath, queryString string) (resp *http.Response, err error) {
	req, err := http.NewRequest(method, API+remotePath+queryString, nil)
	if err != nil {
		return
	}

	date := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	path := fmt.Sprintf("/%s%s", DEFAULT_BUCKET, remotePath)
	msg := strings.Join([]string{method, "", "", date, path}, "\n")
	mac := hmac.New(sha1.New, []byte(SECRET))
	mac.Write([]byte(msg))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	auth := fmt.Sprintf("OSS %s:%s", KEY, signature)

	req.Header.Set("Authorization", auth)
	req.Header.Set("Date", date)

	client := &http.Client{}
	resp, err = client.Do(req)
	return
}

func getListWithMarker(prefix, marker string, files *[]File) (err error) {
	queryString := "?max-keys=1000&prefix=" + prefix
	if marker != "" {
		queryString += "&marker=" + marker
	}
	resp, err := request("GET", "/", queryString)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err = errors.New("OSS API returns: " + resp.Status)
		return
	}
	var list List
	if err = xml.NewDecoder(resp.Body).Decode(&list); err != nil {
		return
	}
	prefixLen := len(prefix)
	for i, _ := range list.Files {
		list.Files[i].Name = list.Files[i].Name[prefixLen:]
	}
	*files = append(*files, list.Files...)
	if !lessVerbose {
		fmt.Fprintf(os.Stderr, "Remote: received %d file names (out of %d) ...\n", len(list.Files), len(*files))
	}
	if list.IsTruncated {
		getListWithMarker(prefix, list.NextMarker, files)
	}
	return
}

func getList(prefix string) (files []File, err error) {
	err = getListWithMarker(prefix, "", &files)
	return
}

func walkFiles(root string) (files []File, err error) {
	rootLen := len(root)
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			var etag string
			if checkMD5 {
				file, err := ioutil.ReadFile(path)
				etag = fmt.Sprintf("\"%X\"", md5.Sum(file))
				if err != nil {
					return err
				}
			}
			files = append(files, File{
				Name: path[rootLen:],
				Size: info.Size(),
				ETag: etag,
			})
		}
		return nil
	})
	return
}

func diff(left, right []File) (ret []File) {
	leftLen, rightLen, retLen := len(left), len(right), 0
	for i := 0; i < leftLen; i++ {
		j, k := 0, 0
		for j < rightLen {
			if right[j].Name == left[i].Name &&
				(!checkMD5 || right[j].ETag == left[i].ETag) {
				break
			}
			j++
		}
		for k < retLen {
			if ret[k].Name == left[i].Name {
				break
			}
			k++
		}
		if j == rightLen && k == retLen {
			ret = append(ret, left[i])
			retLen++
		}
	}
	return
}

var reverseStdoutStderr, checkMD5, lessVerbose bool
var LOCAL, REMOTE string

func init() {
	flag.BoolVar(&reverseStdoutStderr, "r", false, "")
	flag.BoolVar(&reverseStdoutStderr, "remote", false, "")
	flag.BoolVar(&checkMD5, "m", false, "")
	flag.BoolVar(&checkMD5, "md5", false, "")
	flag.BoolVar(&lessVerbose, "s", false, "")
	flag.BoolVar(&lessVerbose, "shhh", false, "")
	flag.Usage = func() {
		fmt.Println("oss-diff [OPTION] LOCAL-DIR  REMOTE-DIR")
		fmt.Println("                  LOCAL-FILE REMOTE-FILE")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("    -r, --reverse  Print LOCAL file paths to stderr, REMOTE to stdout")
		fmt.Println()
		fmt.Println("    -m, --md5      Verify MD5 checksum")
		fmt.Println("    -s, --shhh     Show only file path")
		fmt.Println()
		fmt.Println("Status code: 0 - local and remote are identical")
		fmt.Println("             1 - local has different files")
		fmt.Println("             2 - remote has different files")
		fmt.Println("             3 - both local and remote have different files")
	}
	flag.Parse()
}

func main() {
	timeStart := time.Now()

	if len(flag.Args()) < 2 {
		fmt.Fprintln(os.Stderr, "Error: Please specify local and remote.")
		os.Exit(4)
	}
	if len(flag.Args()) > 2 {
		fmt.Fprintln(os.Stderr, "Error: Please specify one local and remote.")
		os.Exit(4)
	}
	LOCAL = flag.Arg(0)
	REMOTE = flag.Arg(1)

	API = DEFAULT_API_PREFIX
	if strings.Count(API, "%s") == 1 {
		API = fmt.Sprintf(API, DEFAULT_BUCKET)
	}

	var localFiles, remoteFiles []File
	var localFilesLength, remoteFilesLength int
	var localTimeUsed, remoteTimeUsed time.Duration
	var err error

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		timeStart := time.Now()
		localFiles, err = walkFiles(LOCAL)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
		localFilesLength = len(localFiles)
		localTimeUsed = time.Since(timeStart)
		wg.Done()
	}()

	go func() {
		timeStart := time.Now()
		remoteFiles, err = getList(REMOTE)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
		remoteFilesLength = len(remoteFiles)
		remoteTimeUsed = time.Since(timeStart)
		wg.Done()
	}()

	wg.Wait()

	stdout, stderr := os.Stdout, os.Stderr
	if reverseStdoutStderr {
		stdout, stderr = stderr, stdout
	}

	localOnly := diff(localFiles, remoteFiles)
	localOnlyLen := len(localOnly)
	if !lessVerbose {
		fmt.Fprintf(os.Stderr, "Local: %d files, %d different on local\n", localFilesLength, localOnlyLen)
	}
	for i := range localOnly {
		fmt.Fprintln(stdout, LOCAL+localOnly[i].Name)
	}

	remoteOnly := diff(remoteFiles, localFiles)
	remoteOnlyLen := len(remoteOnly)
	if !lessVerbose {
		fmt.Fprintf(os.Stderr, "Remote: %d files, %d different on remote\n", remoteFilesLength, remoteOnlyLen)
	}
	for i := range remoteOnly {
		fmt.Fprintln(stderr, REMOTE+remoteOnly[i].Name)
	}

	retCode := 4
	if localOnlyLen == 0 && remoteOnlyLen == 0 {
		if !lessVerbose {
			fmt.Fprintln(os.Stderr, "Local and remote are identical")
		}
		retCode = 0
	} else if localOnlyLen > 0 && remoteOnlyLen == 0 {
		retCode = 1
	} else if localOnlyLen == 0 && remoteOnlyLen > 0 {
		retCode = 2
	} else if localOnlyLen > 0 && remoteOnlyLen > 0 {
		retCode = 3
	}

	if !lessVerbose {
		fmt.Fprintf(
			os.Stderr,
			"Time used:  %s local  %s remote  %s total\n",
			localTimeUsed.String(),
			remoteTimeUsed.String(),
			time.Since(timeStart).String(),
		)
	}

	os.Exit(retCode)
}