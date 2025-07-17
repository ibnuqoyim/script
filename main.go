package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

var imgRegex = regexp.MustCompile(`https://img-id\.gmbr\.pro/uploads/manga-images/.+/chapter-(\d+)/(\d+)\.jpg`)

func fetchImages(url, outDir string, wg *sync.WaitGroup) ([]string, string, error) {
	defer wg.Done()

	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, "", err
	}

	var downloaded []string
	var chapter string
	var imgWg sync.WaitGroup

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists {
			return
		}

		match := imgRegex.FindStringSubmatch(src)
		if len(match) == 3 {
			chapter = match[1]
			num := match[2]
			filename := fmt.Sprintf("cha%s_%s.jpg", chapter, num)
			imgPath := filepath.Join(outDir, filename)

			imgWg.Add(1)
			go func(url, path string) {
				defer imgWg.Done()
				err := downloadFile(url, path)
				if err == nil {
					fmt.Println("✅ Downloaded:", path)
					downloaded = append(downloaded, path)
				}
			}(src, imgPath)
		}
	})

	imgWg.Wait()
	return downloaded, chapter, nil
}

func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func zipFiles(files []string, zipPath string) error {
	zipfile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	zipWriter := zip.NewWriter(zipfile)
	defer zipWriter.Close()

	for _, file := range files {
		if err := addFileToZip(zipWriter, file); err != nil {
			return err
		}
	}
	return nil
}

func addFileToZip(zipWriter *zip.Writer, filename string) error {
	fileToZip, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fileToZip.Close()

	info, err := fileToZip.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = filepath.Base(filename)
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, fileToZip)
	return err
}

func main() {
	var url string
	fmt.Print("Masukkan URL halaman komik: ")
	fmt.Scan(&url)

	tmpDir := "images"
	os.MkdirAll(tmpDir, 0755)

	var wg sync.WaitGroup
	wg.Add(1)
	files, chapter, err := fetchImages(url, tmpDir, &wg)
	wg.Wait()

	if err != nil {
		fmt.Println("❌ Gagal mengambil gambar:", err)
		return
	}

	if len(files) == 0 {
		fmt.Println("⚠️ Tidak ada gambar ditemukan.")
		return
	}

	zipName := fmt.Sprintf("chapter-%s.zip", chapter)
	zipPath := filepath.Join(".", zipName)

	err = zipFiles(files, zipPath)
	if err != nil {
		fmt.Println("❌ Gagal membuat zip:", err)
		return
	}

	fmt.Printf("✅ ZIP berhasil dibuat: %s\n", zipPath)
}
