package help

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/hypersleep/easyssh"
	"github.com/tj/go-spin"
	"github.com/xshellinc/tools/dialogs"
	"github.com/xshellinc/tools/lib/sudo"
	pb "gopkg.in/cheggaaa/pb.v1"
)

type Iface struct {
	Name         string
	HardwareAddr string
	Ipv4         string
}

type BackgroundJob struct {
	Progress chan bool
	Err      chan error
}

const (
	SshExtendedCommandTimeout = 300
	SshCommandTimeout         = 30

	DefaultSshPort = "22"
)

// Gets homedir based on Os
func UserHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

// Returns the string separator
func Separator() string {
	return string(filepath.Separator)
}

// Returns Os dependent separator
func Separators(os string) string {
	switch os {
	case "linux":
		return string(filepath.Separator)
	default:
		return ""
	}
}

func ExecStandardStd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// Executes commands via sudo
func ExecSudo(cb sudo.PasswordCallback, cbData interface{}, script ...string) (string, error) {
	out, eut, err := sudo.Exec(cb, cbData, script...)
	LogCmdErrors(string(out), string(eut), err, script...)
	if err != nil {
		return string(append(out, eut...)), err
	}
	return string(out), err
}

// Executes command
func ExecCmd(cmdName string, cmdArgs []string) (string, error) {
	cmd := exec.Command(cmdName, cmdArgs...)
	cmdOutput := &bytes.Buffer{}
	cmdStdErr := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	cmd.Stderr = cmdStdErr
	err := cmd.Run()

	LogCmdErrors(cmdOutput.String(), cmdStdErr.String(), err, cmd.Args...)
	if err != nil {
		return string(append(cmdOutput.Bytes(), cmdStdErr.Bytes()...)), err
	}
	return cmdOutput.String(), err
}

func LogCmdErrors(out, eut string, err error, args ...string) {
	if err != nil {
		log.Error("Error while executing: `", args, "` error msg: `", eut,
			"` go error:", err.Error())
		log.Error("Output:", out)
	}
}

func CreateFile(path string) {
	// detect if file exists
	var _, err = os.Stat(path)
	// create file if not exists
	if os.IsNotExist(err) {
		var file, err = os.Create(path)
		LogError(err)
		defer file.Close()
	}
}

func CreateDir(path string) error {
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		log.Error("Error creating dir: ", path, " error msg:", err.Error())
		return err
	}
	return nil
}

func DeleteFile(path string) error {
	log.Debug("deleting file:", path)
	err := os.Remove(path)
	if err != nil {
		LogError(err)
		return err
	}
	return nil
}

func WriteFile(path string, content string) {
	// open file using READ & WRITE permission
	var file, err = os.OpenFile(path, os.O_RDWR, 0644)
	LogError(err)
	defer file.Close()
	// write some text to file
	_, err = file.WriteString(content)
	LogError(err)
	// save changes
	err = file.Sync()
	LogError(err)
	err = file.Sync()
	LogError(err)
}

func DeleteDir(dir string) error {
	d, err := os.Open(dir)
	log.Debug("DeleteDir func():", "removing dir:", dir)
	if err != nil {
		LogError(err)
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		LogError(err)
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			LogError(err)
			return err
		}
	}
	return nil
}

// DownloadFromUrl downloads target file to destination folder
// create destination dir if does not exist
// download file if does not already exist
// shows progress bar
func DownloadFromUrl(url, destination string) (string, error) {
	var (
		timeout time.Duration = time.Duration(0)
		client  http.Client   = http.Client{Timeout: timeout}
	)
	//tokenize url
	tokens := strings.Split(url, "/")
	//obtain file name
	fileName := tokens[len(tokens)-1]

	// check maybe downloaded file exists and corrupted
	fullFileName := filepath.Join(destination, fileName)
	if _, err := os.Stat(fullFileName); !os.IsNotExist(err) {

		downloadedFileLength, _ := GetFileLength(fullFileName)
		sourceFileLength, _ := GetHTTPFileLength(url)

		if sourceFileLength != downloadedFileLength && sourceFileLength != 0 {
			fmt.Printf("[+] Delete corrupted cached file %s\n", fullFileName)
			DeleteFile(fullFileName)
		}
		// otherwise file has correct length
	}

	fmt.Printf("[+] Downloading %s from %s to %s\n", fileName, url, destination)

	//target file does not exist
	if _, err := os.Stat(fmt.Sprintf("%s/%s", destination, fileName)); os.IsNotExist(err) {
		//create destination dir
		CreateDir(destination)
		//create file
		output, err := os.Create(fmt.Sprintf("%s/%s", destination, fileName))
		if err != nil {
			log.Error("[-] Error creating file ", destination, fileName)
			return "", err
		}
		defer output.Close()
		response, err := client.Get(url)
		if err != nil {
			log.Error("[-] Error while downloading file ", url)
			return "", err
		}
		defer response.Body.Close()
		bar := pb.New64(response.ContentLength)
		bar.ShowBar = false
		bar.SetWidth(100)
		bar.Start()
		prd := bar.NewProxyReader(response.Body)
		totalCount, err := io.Copy(output, prd)
		if err != nil {
			log.Error("error while copying ", err.Error())
			return "", err
		}
		log.Debug("Total number of bytes read: ", totalCount)
	} else {
		fmt.Printf("[+] File exist %s%s\n", destination, fileName)
	}
	fmt.Println("\n[+] Done")
	return fileName, nil
}

// DownloadFromUrl downloads target file to destination folder,
// creates destination dir if does not exist
// download file if does not already exist
// shows progress bar
func DownloadFromUrlAsync(url, destination string, readBytesChannel chan int64, errorChan chan error) (string, int64, error) {
	var (
		timeout  time.Duration = time.Duration(0)
		client   http.Client   = http.Client{Timeout: timeout}
		fileName               = "latest"
		length   int64
	)
	response, err := client.Get(url)
	if err != nil {
		log.Error("Error while downloading file from:", url, " error msg:", err.Error())
		return "", 0, err
	}
	contentDisposition := response.Header.Get("Content-Disposition")
	_, params, err := mime.ParseMediaType(contentDisposition)
	if err != nil {
		tokens := strings.Split(url, "/")
		fileName = tokens[len(tokens)-1]
		log.Error("Error parsing media type ", "error msg:", err.Error())
	} else {
		fileName = params["filename"]
	}
	length = response.ContentLength
	// check maybe downloaded file exists
	fullFileName := filepath.Join(destination, fileName)
	if _, err := os.Stat(fullFileName); !os.IsNotExist(err) {
		downloadedFileLength, _ := GetFileLength(fullFileName)
		sourceFileLength, _ := GetHTTPFileLength(url)
		if sourceFileLength > downloadedFileLength && sourceFileLength != 0 {
			log.Debug("Missing bytes. Resuming download")
			go resumeDownloadAsync(fullFileName, url, readBytesChannel, errorChan)
		} else {
			//report full length
			readBytesChannel <- downloadedFileLength
			//close channel
			close(readBytesChannel)
			close(errorChan)
			return fileName, downloadedFileLength, nil
		}
	} else { //file does not exist, download file
		CreateDir(destination)
		//create file
		output, err := os.Create(fmt.Sprintf("%s/%s", destination, fileName))
		if err != nil {
			log.Error("Error creating file ", destination, fileName)
			return "", 0, err
		}
		// ASYNC part. Download file in background and send read bytes into the readBytesChannel channel
		go func(resp *http.Response, ch chan int64, output *os.File, errorChan chan error) {
			defer response.Body.Close()
			defer close(ch)
			defer output.Close()
			defer close(errorChan)
			prd := NewHttpProxyReader(response.Body, func(n int, err error) {
				ch <- int64(n)
				if err != nil {
					log.Error("error occured, sending error down the channel")
					errorChan <- err
					return
				}
			})

			totalCount, err := io.Copy(output, prd)
			if err != nil {
				log.Error("error while copying ", err.Error())
				errorChan <- err
				return
			}
			log.Debug("Total number of bytes read: ", totalCount)

		}(response, readBytesChannel, output, errorChan)
	}
	return fileName, length, nil
}

func resumeDownloadAsync(dst, url string, ch chan int64, errorChan chan error) {
	log.Debug("resuming download to ", dst)
	//resolve redirects to final url
	defer close(ch)
	defer close(errorChan)
	final_url, err := GetFinalUrl(url)
	if err != nil {
		log.Error("http.Get :", err.Error())
	}
	log.Debug("Final resolved URL:", final_url)
	local_length, err := GetFileLength(dst)
	if err != nil {
		log.Error("error getting file length from:", dst, " error msg:", err.Error())
		return
	}
	remote_length, err := GetHTTPFileLength(final_url)
	if err != nil {
		log.Error("error getting remote file length from:", final_url, "error msg:", err.Error())
		return
	}
	log.Debug("Current file size: ", local_length, " remote file length:", remote_length)
	if local_length < remote_length {
		log.Debug("Downloading : ", strconv.FormatInt(remote_length-local_length-1, 10), " bytes")
		//send actual size of the file
		ch <- local_length
		client := &http.Client{}
		req, err := http.NewRequest("GET", final_url, nil)
		if err != nil {
			log.Error("error creating GET request to ", final_url, " error msg:", err.Error())
			return
		}
		range_header := "bytes=" + strconv.FormatInt(local_length, 10) + "-"
		//"-" + strconv.FormatInt(remote_length-1, 10) + "/" + strconv.FormatInt(remote_length, 10)
		req.Header.Add("Range", range_header)
		log.Debug("Adding Range header:", range_header)
		resp, err := client.Do(req)
		defer resp.Body.Close()
		if err != nil {
			log.Error("error making GET request to ", final_url, " error msg:", err.Error())
			return
		}
		log.Debug("Received content length:", resp.ContentLength)
		if resp.StatusCode != http.StatusPartialContent {
			log.Debug("HTTP status code:", resp.StatusCode)
			log.Error("Server does not support Range header, cannot resume download.")
			fmt.Println("\n[-] Server does not support Range header. Cannot resume download. Delete your old file and re-run your command again")
			return
		}

		prd := NewHttpProxyReader(resp.Body, func(n int, err error) {
			ch <- int64(n)
			if err != nil {
				errorChan <- err
			}
		})
		output, err := os.OpenFile(dst, os.O_APPEND|os.O_WRONLY, 0600)
		defer output.Close()
		if err != nil {
			log.Error("error opening file ", "error msg:", err.Error())
		}
		totalCount, err := io.Copy(output, prd)
		if err != nil {
			log.Error("error while copying ", err.Error())
		}
		log.Debug("Total number of bytes read: ", totalCount)
	}
}

func GetFileLength(path string) (int64, error) {
	var file, err = os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return 0, errors.New("File not found " + path)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, errors.New("Can't fetch file info " + path)
	}

	return stat.Size(), nil

}

func GetHTTPFileLength(url string) (int64, error) {
	var (
		timeout time.Duration = time.Duration(0)
		client  http.Client   = http.Client{Timeout: timeout}
	)

	response, err := client.Get(url)
	if err != nil {
		return 0, errors.New("Can't retrieve length of http source " + url)
	}

	return response.ContentLength, nil
}

func GetFinalUrl(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	return resp.Request.URL.String(), nil
}

// Downloads From url with retries
func DownloadFromUrlWithAttempts(url, destination string, retries int) (string, error) {
	var (
		err      error
		filename string
	)
	for i := 1; i <= retries; i++ {
		fmt.Printf("[+] Attempting to download. Trying %d out of %d \n", i, retries)
		filename, err = DownloadFromUrl(url, destination)
		if err == nil {
			break
		} else {
			DeleteFile(filepath.Join(destination, filename))
		}
	}
	if err != nil {
		fmt.Printf("[-] Could not download from url:%s \n", url)
		fmt.Printf("[-] Reported error message:%s\n", err.Error())
		return "", err
	}
	return filename, nil

}

// Downloads from the url asynchronously
func DownloadFromUrlWithAttemptsAsync(url, destination string, retries int, wg *sync.WaitGroup) (string, *pb.ProgressBar, error) {
	var (
		err                     error
		filename                string
		destinationFileNameFull string
		length                  int64
		readBytesChannel        = make(chan int64, 10000)
		errorChan               = make(chan error, 1)
	)
	bar := pb.New64(0)
	bar.ShowBar = false
	bar.SetUnits(pb.U_BYTES)
	wg.Add(1)
	go func(ch chan int64, wg *sync.WaitGroup, errorChan chan error, filename *string, url string) {
		// complete task after closing channel
		defer wg.Done()
		for {
			select {
			case n, ok := <-ch:
				if !ok {
					ch = nil
					log.Debug("Bytes chan is closed")
				}
				bar.Add64(n)
			case err, ok := <-errorChan:
				if !ok {
					errorChan = nil
					log.Debug("Error chan is closed")
				}
				if err != nil {
					// compare length of files, if length is the same - file is downloaded
					fileLength, _ := GetFileLength(*filename)
					urlFileLength, _ := GetHTTPFileLength(url)
					fmt.Println(*filename, url)
					fmt.Println(fileLength, urlFileLength)
					if fileLength != urlFileLength {
						fmt.Println("[-] Error occured with error message:", err.Error())
						log.Error("Error occured while downloading remote file ", "error msg:", err.Error())
						os.Exit(1)
					}
				}
			}
			if ch == nil && errorChan == nil {
				break
			}

		}

	}(readBytesChannel, wg, errorChan, &destinationFileNameFull, url)

	if filename, length, err = DownloadFromUrlAsync(url, destination, readBytesChannel, errorChan); err != nil {
		fmt.Printf("[-] Could not download from url:%s \n", url)
		fmt.Printf("[-] Reported error message:%s\n", err.Error())
		close(readBytesChannel)
		close(errorChan)
		return "", nil, err
	} else {
		bar.Total = length
	}

	destinationFileNameFull = path.Join(destination, filename)

	return filename, bar, nil

}

// Unzip into the destination folder
func Unzip(src, dest string) error {
	fmt.Printf("[+] Unzipping %s to %s\n", src, dest)
	tokens := strings.Split(src, "/")
	fileName := tokens[len(tokens)-1]
	// create destination dir with 0777 access rights
	CreateDir(dest)

	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}

	var total64 int64
	for _, file := range r.File {
		total64 += int64(file.UncompressedSize64)
	}
	bar := pb.New64(total64).SetUnits(pb.U_BYTES)
	bar.ShowBar = false
	bar.SetMaxWidth(80)
	bar.Prefix(fmt.Sprintf("[+] Unzipping %-15s", fileName))
	for _, f := range r.File {
		bar.Start()
		rc, err := f.Open()
		if err != nil {
			fmt.Println("[-] ", err.Error())
			return err
		}
		defer rc.Close()
		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, f.Mode())
		} else {
			var fdir string
			if lastIndex := strings.LastIndex(fpath, string(os.PathSeparator)); lastIndex > -1 {
				fdir = fpath[:lastIndex]
			}
			err = os.MkdirAll(fdir, f.Mode())
			if err != nil {
				fmt.Println("[-] ", err.Error())
				return err
			}
			dst_f, err := os.OpenFile(
				fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				fmt.Println("[-] ", err.Error())
				return err
			}
			defer dst_f.Close()
			copied, err := io.Copy(dst_f, rc)
			if err != nil {
				fmt.Println("[-] ", err.Error())
				return err
			}
			bar.Add64(copied)
		}
	}
	bar.Finish()
	time.Sleep(time.Second * 2)
	fmt.Print("[+] Done\n")
	return nil
}

// Appends a string to the provided file
func AppendToFile(s, target string) error {
	fmt.Printf("[+] Appending %s to %s \n", s, target)
	fileHandle, err := os.OpenFile(target, os.O_APPEND|os.O_RDWR, 0660)
	if err != nil {
		fmt.Printf("[-] Error while appending:%s ", err.Error())
		return err
	}
	defer fileHandle.Close()

	_, err = fileHandle.WriteString(s)
	return err
}

// Writes a string to the provided file
func WriteToFile(s, target string) error {
	fileHandle, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0660)
	if err != nil {
		return err
	}
	defer fileHandle.Close()
	_, err = fileHandle.WriteString(s)

	return err
}

// Get local interfaces with inited ip
func LocalIfaces() ([]Iface, error) {
	var i = make([]Iface, 1)

	ifaces, err := net.Interfaces()

	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {

			var (
				ip   net.IP
				face Iface
			)

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if !ip.IsLoopback() && ip.To4() != nil {
				face.Ipv4 = ip.To4().String()
				face.HardwareAddr = iface.HardwareAddr.String()
				face.Name = iface.Name
				i = append(i, face)
			}
		}
	}

	return i, nil
}

func GetIface() Iface {
	ifaces, _ := LocalIfaces()
	return ifaces[0]
}

// Stream easy ssh
func StreamEasySsh(ip, user, password, port, key, command string, timeout int) (chan string, chan string, chan bool, error) {
	ssh := &easyssh.MakeConfig{
		User:     user,
		Password: password,
		Port:     port,
		Server:   ip,
		Key:      key,
	}

	return ssh.Stream(fmt.Sprintf("sudo %s", command), timeout)
}

// Scp file
func ScpWPort(src, dst, ip, port, user, password string) error {
	ssh := &easyssh.MakeConfig{
		User:     user,
		Password: password,
		Port:     port,
		Server:   ip,
		Key:      "~/.ssh/id_rsa.pub",
	}

	fileName := FileName(src)
	err := ssh.Scp(src, fileName)
	if err != nil {
		return err
	}

	out, err := GenericRunOverSsh(fmt.Sprintf("mv ~/%s %s", fileName, dst), ip, user, password, port, true, false, SshCommandTimeout)
	if err != nil {
		return errors.New(out)
	}

	return nil
}

// Scp file using 22 port
func Scp(src, dst, ip, user, password string) error {
	return ScpWPort(src, dst, ip, "22", user, password)
}

// Generic command run over ssh, which configures ssh detail and calls RunSshWithTimeout method
func GenericRunOverSsh(command, ip, user, password, port string, sudo bool, verbose bool, timeout int) (string, error) {
	ssh := &easyssh.MakeConfig{
		User:     user,
		Password: password,
		Port:     port,
		Server:   ip,
		Key:      "~/.ssh/id_rsa.pub",
	}

	//if sudo {
	//	command = "sudo " + command
	//}

	if sudo && password != "" {
		command = fmt.Sprintf("echo %s | sudo -S %s", password, command)
	}

	if verbose {
		fmt.Printf("[+] Executing %s %s@%s\n", fmt.Sprintf("sudo %s", command), user, ip)
	}

	out, eut, t, err := ssh.Run(command, timeout)
	if !t {
		fmt.Println("[-] Timeout running command : ", command)
		answ := dialogs.YesNoDialog("Would you like to re-run with extended timeout? ")

		if answ {
			out, eut, t, err = ssh.Run(command, SshExtendedCommandTimeout)

			if !t {
				fmt.Println("[-] Timeout running command : ", command)
				return out, errors.New(eut)
			}
		}
	}

	if err != nil {
		fmt.Println("[-] Error running command : ", command, " err msg:", eut)
	}

	return out, nil
}

// Run ssh echo password | sudo command with timeout
func RunSudoOverSshTimeout(command, ip, user, password string, timeout int) (string, error) {
	return GenericRunOverSsh(command, ip, user, password, DefaultSshPort, true, false, timeout)
}

// Run ssh echo password | sudo command
func RunSudoOverSsh(command, ip, user, password string, verbose bool) (string, error) {
	return GenericRunOverSsh(command, ip, user, password, DefaultSshPort, true, verbose, SshCommandTimeout)
}

// Run ssh command
func RunOverSsh(command, ip, user, password string) (string, error) {
	return GenericRunOverSsh(command, ip, user, password, DefaultSshPort, false, false, SshCommandTimeout)
}

// Copy a file
func Copy(src, dst string) error {
	sourcefile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourcefile.Close()
	destfile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destfile.Close()

	if _, err = io.Copy(destfile, sourcefile); err != nil {
		return err
	}
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, sourceInfo.Mode())
}

// Copy a directory recursively
func CopyDir(src, dst string) error {
	// get properties of source dir
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	// create dest dir
	if err = os.MkdirAll(dst, sourceInfo.Mode()); err != nil {
		return err
	}

	directory, _ := os.Open(src)
	defer directory.Close()

	objects, err := directory.Readdir(-1)
	for _, obj := range objects {
		srcp := src + Separator() + obj.Name()
		dstp := dst + Separator() + obj.Name()
		if obj.IsDir() {
			// create sub-directories recursively
			err = CopyDir(srcp, dstp)
			if err != nil {
				fmt.Println(err)
			}
			continue
		}

		// perform copy
		err = Copy(srcp, dstp)
		if err != nil {
			fmt.Println(err)
		}
	}
	return nil
}

// Returns an absolute path of the path
func Abs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		log.Error("Error getting absolute path ", "err msg:", err.Error())
		return ""
	}
	return abs
}

// Returns a default bin path
func GetBinPath() string {
	switch runtime.GOOS {
	case "linux":
		return "/usr/local/bin"
	case "darwin":
		return "/usr/local/bin"
	default:
		return ""
	}
}

// DirExists returns whether the given file or directory exists or not
func DirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// return true if the given file is writable/readable/executable using the given mask by an owner
func FileModeMask(name string, mask os.FileMode) (bool, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return false, err
	}

	return fi.Mode()&mask == mask, nil
}

// Gets an exit code from the error
func CommandExitCode(e error) (int, error) {
	if ee, ok := e.(*exec.ExitError); ok {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
			return ws.ExitStatus(), nil
		}
	}

	return 0, errors.New("Wrong error type")
}

// Gets a filename from the path
func FileName(path string) string {
	split := strings.Split(path, string(os.PathSeparator))
	name := split[len(split)-1]
	return name
}

func StringToSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}

	return false
}

// Shows the message and rotates a spinner while `progress` is true and isn't closed
func WaitAndSpin(message string, progress chan bool) {
	s := spin.New()
	s.Set(spin.Spin1)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	spinEn := true
	ok := false

Loop:
	for {
		select {
		case spinEn, ok = <-progress:
			if !ok {
				fmt.Print("\n")
				break Loop
			}
		case <-ticker.C:
			if spinEn {
				fmt.Printf("\r[+] %s: %s ", message, s.Next())
			}
		}
	}
}

// TODO should replace WaitAndSpin
func NewBackgroundJob() *BackgroundJob {
	return &BackgroundJob{
		Progress: make(chan bool),
		Err:      make(chan error),
	}
}

func (b *BackgroundJob) Error(err error) {
	b.Err <- err
}

func (b *BackgroundJob) Active(active bool) {
	b.Progress <- active
}

func (b *BackgroundJob) Close() {
	close(b.Progress)
}

func WaitJobAndSpin(message string, job *BackgroundJob) (err error) {
	s := spin.New()
	s.Set(spin.Spin1)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	spinEn := true
	ok := false

Loop:
	for {
		select {
		case spinEn, ok = <-job.Progress:
			if !ok {
				fmt.Print("\n")
				break Loop
			}
		case err = <-job.Err:
			fmt.Print("\n")
			break Loop

		case <-ticker.C:
			if spinEn {
				fmt.Printf("\r[+] %s: %s ", message, s.Next())
			}
		}
	}

	return
}

// Logs an error if any
func LogError(err error) {
	if err != nil {
		log.Error(err.Error())
	}
}

// Exits with the code 1 in case of any error
func ExitOnError(err error) {
	if err != nil {
		fmt.Println("[-] Error: ", err.Error())
		fmt.Println("[-] Exiting ... ")
		log.Fatal("erro msg:", err.Error())
	}
}

// Checks connection
func EstablishConn(ip, user, passwd string) bool {
	fmt.Printf("[+] Trying to reach %s@%s\n", user, ip)
	ssh := &easyssh.MakeConfig{
		User:     user,
		Server:   ip,
		Password: passwd,
		Port:     "22",
	}
	resp, eut, t, err := ssh.Run("whoami", SshCommandTimeout)
	if err != nil || !t {
		fmt.Printf("[-] Host is unreachable %s@%s err:%s\n", user, ip, eut)
		return false
	} else {
		fmt.Println("[+] Command `whoami` result: ", strings.Trim(resp, "\n"))
		return true
	}
	return false
}
