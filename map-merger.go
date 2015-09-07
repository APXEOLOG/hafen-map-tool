/*
	Created by Artyom Melnikov (APXEOLOG), 2015
 */
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"crypto/md5"
	"encoding/json"
	"time"
	"bytes"
	"container/list"
	"path/filepath"
	"io"
	"strings"
	"image/png"
	"image"
	"image/draw"
	"image/color"
	"strconv"
	"github.com/nfnt/resize"
)

var SESSION_FOLDER string = "sessions"

type MinimapMetaData struct {
	HashSimple []byte
	Filename string
	X int32
	Y int32
}

type SPoint struct {
	X int32
	Y int32
}

type SessionMetaData struct {
	CreationDate int64
	Content []MinimapMetaData
}

type MinimapMetaDataPair struct {
	first MinimapMetaData
	second MinimapMetaData
}

func generateMinimapMetaData(files []os.FileInfo, basePath string) []MinimapMetaData {
	buffer := make([]MinimapMetaData, len(files))
	for i := 0; i < len(files); i++ {
		var x, y int32 = 0, 0
		fmt.Sscanf(files[i].Name(), "tile_%d_%d.png", &x, &y)
		filecontent, _ := ioutil.ReadFile(filepath.Join(basePath, files[i].Name()))
		hash := md5.Sum(filecontent)
		buffer[i] = MinimapMetaData{HashSimple: hash[:], Filename: filepath.Join(basePath, files[i].Name()), X: x, Y: y }
	}
	return buffer
}

// Argument is absolute path to directory
func getSessionMetaData(folder string) SessionMetaData {
	var metadata SessionMetaData
	metaDataFilePath := filepath.Join(folder, "metadata.json")
	data, err := ioutil.ReadFile(metaDataFilePath)
	if err != nil {
		parsedTime, _ := time.Parse("2006-01-02 15.04.05", folder)
		minimaps, _ := ioutil.ReadDir(folder)
		metadata = SessionMetaData{CreationDate: parsedTime.Unix(), Content: generateMinimapMetaData(minimaps, folder)}
		encodedData, _ := json.Marshal(metadata)
		err := ioutil.WriteFile(metaDataFilePath, encodedData, 0777)
		if err != nil {
			fmt.Printf("Error while saving metadata.json: %s\n", err.Error())
		}
	} else {
		json.Unmarshal(data, &metadata)
	}
	return metadata
}


func areSessionsMergeable(source SessionMetaData, destination SessionMetaData) (bool, int32, int32) {
	offsetMap := make(map[SPoint]int16)
	hits := list.New()
	for i:= 0; i < len(source.Content); i++ {
		for j:= 0; j < len(destination.Content); j++ {
			if bytes.Compare(source.Content[i].HashSimple, destination.Content[j].HashSimple) == 0 {
				offset := SPoint{source.Content[i].X - destination.Content[j].X, source.Content[i].Y - destination.Content[j].Y}
				offsetMap[offset] = offsetMap[offset] + 1
				hits.PushBack(MinimapMetaDataPair{source.Content[i], destination.Content[j]})
			}
		}
	}
	if hits.Len() == 0 {
		return false, 0, 0
	}

	var bestOffset SPoint
	var bestCount int16 = 0
	for key, value := range offsetMap {
		if value > 2 && value > bestCount {
			bestCount = value
			bestOffset = key
		}
		fmt.Printf("Offset: %d Count: %d\n", key, value)
	}
	if bestCount == 0 {
		return false, 0, 0
	} else {
		return true, bestOffset.X, bestOffset.Y
	}
}

func CopyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

// Arguments are absolute paths to directory
func mergeFolders(sourcePath string, destinationPath string) bool {
	_, sinfo := os.Stat(sourcePath)
	_, dinfo := os.Stat(destinationPath)
	if os.IsNotExist(sinfo) || os.IsNotExist(dinfo) {
		return false
	}

	sourceMetaData := getSessionMetaData(sourcePath)
	destinationMetaData := getSessionMetaData(destinationPath)
	success, offsetX, offsetY := areSessionsMergeable(sourceMetaData, destinationMetaData)
	if success == true {
		fmt.Printf("Sessions are mergeable (%d, %d)\n", offsetX, offsetY)
		// Sub offset from source and move to dest
		for i:= 0; i < len(sourceMetaData.Content); i++ {
			filePath := filepath.Join(destinationPath, fmt.Sprintf("tile_%d_%d.png", (sourceMetaData.Content[i].X - offsetX), (sourceMetaData.Content[i].Y - offsetY)))
			err := CopyFile(sourceMetaData.Content[i].Filename, filePath);
			if err != nil { fmt.Println("Copy error: " + err.Error()) }
		}
		// Remove source dir
		os.RemoveAll(sourcePath)
		// Remove metadata.json
		os.Remove(filepath.Join(destinationPath, "metadata.json"))
		return true
	} else {
		return false
	}
}

func getImageDimension(imagePath string) (int, int) {
	file, err := os.Open(imagePath)
	if err != nil {
		fmt.Printf("Cannot get image size #1\n")
		return 0, 0
	}
	defer file.Close()
	image, err := png.DecodeConfig(file)
	if err != nil {
		fmt.Printf("Cannot get image size #2: %s\n", err.Error())
		return 0, 0
	}
	return image.Width, image.Height
}

func getImage(basePath string, x int, y int) image.Image {
	file, err := os.Open(filepath.Join(basePath, fmt.Sprintf("tile_%d_%d.png", x, y)))
	if err != nil {
		return nil
	}
	image, err := png.Decode(file)
	if err != nil {
		return nil
	}
	file.Close()
	return image
}

func copySessionFiles(src, dest string) {
	files, err := ioutil.ReadDir(src)
	if err != nil {
		fmt.Printf("Cannot list files: %s\n", err.Error())
		return
	}
	for j := 0; j < len(files); j++ {
		if strings.Contains(files[j].Name(), "metadata") { continue }
		CopyFile(filepath.Join(src, files[j].Name()), filepath.Join(dest, files[j].Name()))
	}
}

func generatePicture(workingDirectory, session string) {
	fmt.Printf("This mode is not supported yet\n")
	return
}

func generateZoom(sourcePath string, outputPath string, tileSize int, composeCount int, resizeToSize bool) {
	metadata := getSessionMetaData(sourcePath)
	fmt.Printf("Tiles: %d\n", len(metadata.Content))
	// Find bounds
	var minX, minY, maxX, maxY, i, j int = 0, 0, 0, 0, 0, 0
	for i = 0; i < len(metadata.Content); i++ {
		if int(metadata.Content[i].X) < minX {
			minX = int(metadata.Content[i].X)
		}
		if int(metadata.Content[i].X) > maxX {
			maxX = int(metadata.Content[i].X)
		}
		if int(metadata.Content[i].Y) < minY {
			minY = int(metadata.Content[i].Y)
		}
		if int(metadata.Content[i].Y) > maxY {
			maxY = int(metadata.Content[i].Y)
		}
	}
	fmt.Printf("Size: %d, %d -> %d, %d\n", minX, minY, maxX, maxY)
	// Generate next zoom level
	for y := int(minY / composeCount) - 1; y <= int(maxY / composeCount) + 1; y++ {
		for x := int(minX / composeCount) - 1; x <= int(maxX / composeCount) + 1; x++ {
			fileP := filepath.Join(outputPath, fmt.Sprintf("tile_%d_%d.png", x, y))
			generatedImage := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{tileSize * composeCount, tileSize * composeCount}})
			transparent := color.RGBA{0, 0, 0, 0}
			draw.Draw(generatedImage, generatedImage.Bounds(), &image.Uniform{transparent}, image.ZP, draw.Src)
			usedTiles := 0
			for j = 0; j < composeCount; j++ {
				for i = 0; i < composeCount; i++ {
					imageZ := getImage(sourcePath, x * composeCount + i, y * composeCount + j)
					if imageZ != nil {
						draw.Draw(generatedImage,
							image.Rectangle{image.Point{i * tileSize, j * tileSize}, image.Point{(i + 1) * tileSize, (j + 1) * tileSize}},
							imageZ,
							image.ZP,
							draw.Src)
						usedTiles++
					}
				}
			}
			if usedTiles == 0 {
				continue
			}
			fileHandle, err := os.OpenFile(fileP, os.O_WRONLY | os.O_TRUNC | os.O_CREATE, 0777)
			if err != nil {
				fmt.Printf("Cannot create zoom file: %s\n", err.Error())
			} else {
				var resized image.Image = generatedImage
				if resizeToSize {
					resized = resize.Resize(uint(tileSize), uint(tileSize), generatedImage, resize.Bilinear)
				}
				png.Encode(fileHandle, resized)
				fileHandle.Close()
			}
		}
	}
}

func generateTiles(workingDirectory, session, outputPath string) {
	dirPath := filepath.Join(workingDirectory, session)

	os.RemoveAll(outputPath)
	err := os.Mkdir(outputPath, 0777)
	if err != nil {
		fmt.Printf("Cannot create output folder (%s): %s\n", outputPath, err.Error())
		return
	}

	// Generate zoom level 5
	zoomedPath := filepath.Join(outputPath, "5")
	os.Mkdir(zoomedPath, 0777)
	tileSize := 100
	composeCount := 4
	generateZoom(dirPath, zoomedPath, tileSize, composeCount, false)

	for zoom := 4; zoom > 0; zoom-- {
		folder := filepath.Join(outputPath, strconv.Itoa(zoom + 1))
		zoomedPath := filepath.Join(outputPath, strconv.Itoa(zoom))
		os.Mkdir(zoomedPath, 0777)
		generateZoom(folder, zoomedPath, tileSize * composeCount, 2, true)
	}
}

func main() {
	var mode, session string = "merger", ""
	// Output folder for zoommode
	var outputFodler string = "zoommap"
	// Remove non-standard sessions
	var removeNonStandard bool = false
	// Session trimming
	var trimSessions bool = false
	var trimSessionsCount int = 0

	// Parse CMD
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-d": SESSION_FOLDER = args[i + 1]
			i++
			break
		case "-z": mode = "zoomer"
			session = args[i + 1]
			i++
			break
		case "-o": outputFodler = args[i + 1]
			i++
			break
		case "-p": mode = "picture"
			session = args[i + 1]
			i++
			break
		case "-t": trimSessions = true
			trimSessionsCount, _ = strconv.Atoi(args[i + 1])
			i++
			break
		case "-c": removeNonStandard = true
			break
		}
	}

	workingDirectory, _ := filepath.Abs(SESSION_FOLDER)

	// Generate zoom levels for specific session
	if mode == "zoomer" {
		generateTiles(workingDirectory, session, outputFodler)
		return
	}

	// Generate single picture for specific session
	if mode == "picture" {
		generatePicture(workingDirectory, session)
		return
	}

	// Otherwise, let's make cross-merge
	files, _ := ioutil.ReadDir(workingDirectory)
	if len(files) < 2 {
		fmt.Println("No folders found")
		return
	}

	if removeNonStandard == true {
		// Remove all sessions with tile size != 100x100
		for j := 0; j < len(files); j++ {
			tiles, _ := ioutil.ReadDir(filepath.Join(workingDirectory, files[j].Name()))
			for i := 0; i < len(tiles); i++ {
				if strings.Contains(tiles[i].Name(), "tile_") {
					sx, sy := getImageDimension(filepath.Join(workingDirectory, files[j].Name(), tiles[i].Name()))
					if sx != 100 || sy != 100 {
						fmt.Printf("Old session removed: %s\n", files[j].Name())
						os.RemoveAll(filepath.Join(workingDirectory, files[j].Name()))
					}
					break
				}
			}
		}
	}

	files, _ = ioutil.ReadDir(workingDirectory)
	if len(files) < 2 {
		fmt.Println("No folders found")
		return
	}
	for j := 0; j < len(files); j++ {
		info, err := os.Stat(filepath.Join(workingDirectory, files[j].Name()))
		if err != nil { continue }
		if info.IsDir() == false { continue }

		coreFolder := files[j]
		for i:= 1; i < len(files); i++ {
			if i == j { continue }
			dirInfo, err := os.Stat(filepath.Join(workingDirectory, files[i].Name()))
			if err != nil { continue }
			if dirInfo.IsDir() == false { continue }

			res := mergeFolders(filepath.Join(workingDirectory, files[i].Name()), filepath.Join(workingDirectory, coreFolder.Name()))
			if res == true {
				fmt.Printf("Merged (%s, %s)\n", coreFolder.Name(), files[i].Name())
			} else {
				fmt.Printf("Sessions are not mergeable (%s, %s)\n",  coreFolder.Name(), files[i].Name())
			}
		}
	}
	files, _ = ioutil.ReadDir(workingDirectory)
	var sessionsJS string = "var sessionsJS = ["
	for j := 0; j < len(files); j++ {
		tiles, _ := ioutil.ReadDir(filepath.Join(workingDirectory, files[j].Name()))
		if trimSessions == true {
			if len(tiles) < trimSessionsCount {
				err := os.RemoveAll(filepath.Join(workingDirectory, files[j].Name()))
				if err != nil {
					fmt.Printf("Cannot trim session %s: %s\n", files[j].Name(), err.Error())
					continue
				} else {
					fmt.Printf("Trimmed session %s\n", files[j].Name())
					continue
				}
			}
		}
		sessionsJS += "\"" + SESSION_FOLDER + "/" + files[j].Name() + "\", "
	}
	sessionsJS += "];"
	ioutil.WriteFile("session.js", []byte(sessionsJS), 0777)
}