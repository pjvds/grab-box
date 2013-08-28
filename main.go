package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"github.com/op/go-logging"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
)

var log = logging.MustGetLogger("main")

var archiveFilename = flag.String("archive", "", "The archive to use. Setting this option will surpress the download.")
var boxName = flag.String("name", "", "The name of the box.")

func main() {
	flag.Parse()

	fmt.Println("Welcome to grab-box\n")

	if archiveFilename == nil || *archiveFilename == "" {
		fmt.Print("What is the url of the box?")

		var boxurl string
		if _, err := fmt.Scanln(&boxurl); err != nil {
			log.Fatal(err)
		}
		fmt.Println("")

		filename, err := downloadBox(boxurl)
		if err != nil {
			log.Fatal(err)
		}

		archiveFilename = &filename
	}

	if boxName == nil || *boxName == "" {
		fmt.Println("What is the name of the box?\n")

		if _, err := fmt.Scanln(boxName); err != nil {
			log.Fatal(err)
		}
	}

	unpackArchive(*archiveFilename, *boxName)

	fmt.Printf("\n\nFinished! You can execute the following command to start the container:\n\n")
	fmt.Printf("\tsudo lxc-start -n '%v'", *boxName)
}

func unpackArchive(filename string, boxname string) {
	containerDir := fmt.Sprintf("/var/lib/lxc/%v", boxname)

	if err := os.MkdirAll(containerDir, os.ModeDir); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nUnarchiving container...")
	untar(filename, containerDir)
}

func fixConfig(filename string) {

}

// Downloads the box archive to a temporary file. It returns the filename
// to the temporary file, or an error.
func downloadBox(url string) (string, error) {
	fmt.Printf("Downloading box: %v\n", url)

	response, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("Unable to create response for url: %v", err)
	}
	defer response.Body.Close()

	archiveFile, err := ioutil.TempFile(os.TempDir(), "box.tar.gz")
	if err != nil {
		return "", fmt.Errorf("Unable to create temporary file: %v", err)
	}
	defer archiveFile.Close()

	percentageComplete := float64(0)
	bytesWritten := int64(0)
	buffer := make([]byte, 32*1024)
	for {
		nRead, errRead := response.Body.Read(buffer)
		if nRead > 0 {
			nWritten, errWrite := archiveFile.Write(buffer[0:nRead])
			if nWritten > 0 {
				bytesWritten += int64(nWritten)
			}
			if errWrite != nil {
				log.Fatalf("Error writing file: %v", err)
			}
			if nRead != nWritten {
				log.Fatal(io.ErrShortWrite)
			}
		}
		if errRead == io.EOF {
			break
		}
		if errRead != nil {
			log.Fatal("Error ")
		}

		newPercentageComplete := (float64(100) / float64(response.ContentLength)) * float64(bytesWritten)
		if math.Floor(newPercentageComplete) != math.Floor(percentageComplete) {
			fmt.Print(".")
			percentageComplete = newPercentageComplete
		}
	}

	fmt.Print("Complet!\n\n")
	return archiveFile.Name(), nil
}

func untar(filename string, directory string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		log.Fatal(err)
	}

	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		if header.FileInfo().IsDir() {
			dir := filepath.Join(directory, header.Name)
			os.Mkdir(dir, os.ModeDir)
		} else {
			path := filepath.Join(directory, header.Name)
			file, err := os.Create(path)
			if err != nil {
				log.Fatal(err)
			}

			if _, err := io.Copy(file, reader); err != nil {
				log.Fatal(err)
			}
		}
	}

	os.Chmod(directory, 0755)
	fmt.Printf("Container created in: %v", directory)
}
