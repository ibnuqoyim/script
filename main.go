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

func fetchImages(url, outDir string) ([]string, string, error) {
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
	var wg sync.WaitGroup
	lock := &sync.Mutex{}

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists {
			return
		}

		match := imgRegex.FindStringSubmatch(src)
		if len(match) == 3 {
			chapter = match[1]
			imgNum := match[2]
			imgNumPadded := fmt.Sprintf("%02s", imgNum)

			filename := fmt.Sprintf("cha%s_%s.jpg", chapter, imgNumPadded)
			imgPath := filepath.Join(outDir, filename)

			wg.Add(1)
			go func(url, path string) {
				defer wg.Done()
				if err := downloadFile(url, path); err == nil {
					fmt.Println("✅", path)
					lock.Lock()
					downloaded = append(downloaded, path)
					lock.Unlock()
				}
			}(src, imgPath)
		}
	})

	wg.Wait()
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
	var baseURL string
	var startCh, endCh int

	fmt.Print("Masukkan base URL (misal https://site.com/manga/chapter-): ")
	fmt.Scan(&baseURL)

	fmt.Print("Mulai dari chapter: ")
	fmt.Scan(&startCh)

	fmt.Print("Sampai chapter: ")
	fmt.Scan(&endCh)

	for ch := startCh; ch <= endCh; ch++ {
		chapterURL := fmt.Sprintf("%schapter-%d", baseURL, ch)
		fmt.Println("🔎 Memproses:", chapterURL)

		tmpDir := fmt.Sprintf("images_ch%d", ch)
		os.MkdirAll(tmpDir, 0755)

		files, chapterStr, err := fetchImages(chapterURL, tmpDir)
		if err != nil {
			fmt.Println("❌ Gagal:", err)
			continue
		}
		if len(files) == 0 {
			fmt.Println("⚠️  Tidak ada gambar ditemukan.")
			continue
		}

		zipName := fmt.Sprintf("chapter-%s.zip", chapterStr)
		err = zipFiles(files, zipName)
		if err != nil {
			fmt.Println("❌ Gagal zip:", err)
			continue
		}

		fmt.Printf("✅ Chapter %s selesai → %s\n", chapterStr, zipName)
	}
}
