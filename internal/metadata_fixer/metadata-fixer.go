package metadata_fixer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
	jis "github.com/dsoprea/go-jpeg-image-structure/v2"

	ifd_types "github.com/opdude/google-photo-exporter-metadata-fixer/internal/ifd"
	googleexportjson "github.com/opdude/google-photo-exporter-metadata-fixer/pkg/google-export-json"
)

type EXIFGPSData struct {
	LatitudeRef  rune
	Latitude     []exifcommon.Rational
	LongitudeRef rune
	Longitude    []exifcommon.Rational
	Altitude     []exifcommon.Rational
	AltitudeRef  int32
	Timestamp    []exifcommon.Rational
	Datestamp    string
	GPSInfo      *exif.GpsInfo
}

type UnsupportedFileType struct {
	FileType string
	Err      error
}

func (r *UnsupportedFileType) Error() string {
	return fmt.Sprintf("unsupportedFileType %s: err %v", r.FileType, r.Err)
}

func FixPhotoMetadata(src string, removeJSON bool) error {
	jsonFiles := findAllJSONFilesInDir(src)

	removeJSONFunc := func(file string) error {
		if removeJSON {
			err := removeJSONFile(file)
			if err != nil {
				return err
			}
		}

		return nil
	}

	for _, jsonFilePath := range jsonFiles {
		jsonFile, err := os.Open(jsonFilePath)
		if err != nil {
			fmt.Println(err)
		}
		defer jsonFile.Close()

		byteValue, _ := ioutil.ReadAll(jsonFile)

		var result googleexportjson.GoogleExportJSON

		fmt.Println("Found: " + jsonFilePath)
		json.Unmarshal([]byte(byteValue), &result)
		err, hasDiff := photoHasEXIFDifference(filepath.Dir(jsonFilePath), result)

		// Skip unsupported file errors
		_, ok := err.(*UnsupportedFileType)
		if ok {
			fmt.Println("Skipping unsupported file: " + jsonFilePath)
			err = removeJSONFunc(jsonFile.Name())
			if err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}

		if !hasDiff {
			fmt.Println("Skipping: " + jsonFilePath)
			err = removeJSONFunc(jsonFile.Name())
			if err != nil {
				return err
			}
			continue
		}

		fmt.Println("Processing: " + jsonFilePath)
		err = updatePhotoDateAndEXIF(filepath.Dir(jsonFilePath), result)
		if err != nil {
			return err
		}

		err = removeJSONFunc(jsonFile.Name())
		if err != nil {
			return err
		}

	}

	return nil
}

func photoHasEXIFDifference(baseDir string, jsonData googleexportjson.GoogleExportJSON) (error, bool) {

	var segmentList *jis.SegmentList

	// check if file exists
	if _, err := os.Stat(filepath.Join(baseDir, jsonData.Title)); os.IsNotExist(err) {
		fmt.Println("File does not exist: " + filepath.Join(baseDir, jsonData.Title))
		return nil, false
	}

	// Handle different file types
	switch strings.ToLower(filepath.Ext(jsonData.Title)) {
	case ".jpg":
		// handle jpg files
		photoParser, _ := jis.NewJpegMediaParser().ParseFile(filepath.Join(baseDir, jsonData.Title))
		segmentList = photoParser.(*jis.SegmentList)
	default:
		return &UnsupportedFileType{FileType: filepath.Ext(jsonData.Title)}, false
	}

	// From our segment list, we can now extract the EXIF data rootIb
	rootIb, err := segmentList.ConstructExifBuilder()

	// Current EXIF data isn't healthy so we skip
	if err != nil {
		return nil, false
	}

	err, hasDiff := hasValueDiff(rootIb, segmentList, jsonData)
	if err != nil {
		return err, false
	}

	if hasDiff {
		return nil, true
	}

	return nil, false
}

func hasValueDiff(rootIb *exif.IfdBuilder, segmentList *jis.SegmentList, jsonData googleexportjson.GoogleExportJSON) (error, bool) {

	gpsValues, err := getGPSValues(jsonData)
	if err != nil {
		return err, false
	}

	ifdComparisons := map[string]map[string]interface{}{
		ifd_types.IfdPathStandard: {
			"ImageDescription": jsonData.Description,
		},
		ifd_types.IfdPathStandardExif: {
			"DateTimeOriginal":  exifcommon.ExifFullTimestampString(time.Time(jsonData.PhotoTakenTime.Timestamp)),
			"DateTimeDigitized": exifcommon.ExifFullTimestampString(time.Time(jsonData.CreationTime.Timestamp)),
		},
	}

	for ifdPath, ifdPathTags := range ifdComparisons {
		childIb, err := exif.GetOrCreateIbFromRootIb(rootIb, ifdPath)
		if err != nil {
			return err, false
		}

		for tagName, expectedValue := range ifdPathTags {
			// Skip if we have an empty value
			if expectedValue == "" || expectedValue == nil {
				continue
			}

			currentValueStr := ""
			currentValue, err := childIb.FindTagWithName(tagName)
			if currentValue != nil {
				currentValueStr = string(bytes.Trim(currentValue.Value().Bytes(), "\x00")) // Trim null bytes from the string value
			}

			if err != nil && !errors.Is(err, exif.ErrTagEntryNotFound) {
				return err, false
			}
			if string(currentValueStr) != expectedValue {
				return nil, true
			}
		}
	}

	//If we don't have any GPS data in our JSON then we assume GPS data is the same
	if gpsValues.GPSInfo == nil {
		return nil, false
	}

	// Check GPS values, this requires a different approach as we need to check the raw EXIF data
	_, rawExif, err := segmentList.Exif()
	if err != nil {
		return err, false
	}
	im := exifcommon.NewIfdMapping()
	err = exifcommon.LoadStandardIfds(im)
	if err != nil {
		return err, false
	}

	ti := exif.NewTagIndex()

	_, index, err := exif.Collect(im, ti, rawExif)
	if err != nil {
		return err, false
	}

	ifd, err := index.RootIfd.ChildWithIfdPath(exifcommon.IfdGpsInfoStandardIfdIdentity)

	// We don't have any GPS data so it's different
	if errors.Is(err, exif.ErrTagNotFound) {
		return nil, true
	}
	if err != nil {
		return err, false
	}

	gi, err := ifd.GpsInfo()
	// GPS data fails to be parsed so we should update it
	if err != nil {
		return nil, true
	}

	if !reflect.DeepEqual(gi, gpsValues.GPSInfo) {
		return nil, true
	}

	// We didn't find any differences
	return nil, false
}

// updatePhotoDateAndEXIF() function is used to update the given photos date and EXIF data based on the JSON data provided
// the photo is loaded and then the date and EXIF data is updated and the file saved back to the same location
func updatePhotoDateAndEXIF(baseDir string, jsonData googleexportjson.GoogleExportJSON) error {

	var segmentList *jis.SegmentList

	// Handle different file types
	switch strings.ToLower(filepath.Ext(jsonData.Title)) {
	case ".jpg":
		// handle jpg files
		photoParser, _ := jis.NewJpegMediaParser().ParseFile(filepath.Join(baseDir, jsonData.Title))
		segmentList = photoParser.(*jis.SegmentList)
	default:
		return fmt.Errorf("unsupported file type: %s", filepath.Ext(jsonData.Title))
	}

	// From our segment list, we can now extract the EXIF data rootIb
	rootIb, err := segmentList.ConstructExifBuilder()
	if err != nil {
		return err
	}

	// For each IFD child set all the values we have in the JSON data
	ifdChildren := []string{ifd_types.IfdPathStandard, ifd_types.IfdPathStandardExif, ifd_types.IfdPathStandardGps}
	for _, ifdChild := range ifdChildren {
		err = setValues(ifdChild, rootIb, jsonData)
		if err != nil {
			return err
		}
	}

	// Set the updated EXIF data back to the segment list
	err = segmentList.SetExif(rootIb)
	if err != nil {
		return err
	}

	// Write the updated EXIF data back to the file
	f, _ := os.OpenFile(filepath.Join(baseDir, jsonData.Title), os.O_RDWR, 0644)
	defer f.Close()
	err = segmentList.Write(f)
	if err != nil {
		return err
	}

	return nil
}

func setValues(ifdPath string, rootIb *exif.IfdBuilder, jsonData googleexportjson.GoogleExportJSON) error {
	childIb, err := exif.GetOrCreateIbFromRootIb(rootIb, ifdPath)
	if err != nil {
		return err
	}

	switch ifdPath {
	case ifd_types.IfdPathStandard:
		err := childIb.SetStandardWithName("ImageDescription", jsonData.Description)
		if err != nil {
			return err
		}

	case ifd_types.IfdPathStandardExif:
		err = childIb.SetStandardWithName("DateTimeOriginal", exifcommon.ExifFullTimestampString(time.Time(jsonData.PhotoTakenTime.Timestamp)))
		if err != nil {
			return err
		}

		err = childIb.SetStandardWithName("DateTimeDigitized", exifcommon.ExifFullTimestampString(time.Time(jsonData.CreationTime.Timestamp)))
		if err != nil {
			return err
		}

	case ifd_types.IfdPathStandardGps:
		gpsValues, err := getGPSValues(jsonData)
		if err != nil {
			return err
		}

		err = childIb.SetStandardWithName("GPSVersionID", []byte{byte(2), byte(2), byte(0), byte(0)})
		if err != nil {
			return err
		}
		err = childIb.SetStandardWithName("GPSLatitudeRef", []byte{byte(gpsValues.LatitudeRef)})
		if err != nil {
			return err
		}
		err = childIb.SetStandardWithName("GPSLatitude", gpsValues.Latitude)
		if err != nil {
			return err
		}
		err = childIb.SetStandardWithName("GPSLongitudeRef", []byte{byte(gpsValues.LongitudeRef)})
		if err != nil {
			return err
		}
		err = childIb.SetStandardWithName("GPSLongitude", gpsValues.Longitude)
		if err != nil {
			return err
		}
		err = childIb.SetStandardWithName("GPSAltitude", gpsValues.Altitude)
		if err != nil {
			return err
		}
		err = childIb.SetStandardWithName("GPSAltitudeRef", []byte{byte(gpsValues.AltitudeRef)})
		if err != nil {
			return err
		}
		err = childIb.SetStandardWithName("GPSTimeStamp", gpsValues.Timestamp)
		if err != nil {
			return err
		}
		err = childIb.SetStandardWithName("GPSDateStamp", []byte(gpsValues.Datestamp))
		if err != nil {
			return err
		}
	}

	return nil
}

func getGPSValues(jsonData googleexportjson.GoogleExportJSON) (gpsData EXIFGPSData, err error) {
	var jsonGPSData = jsonData.GeoData
	if jsonData.GeoData.Longitude == 0 {
		jsonGPSData = jsonData.GeoDataExif
	}

	// Skip as we don't have any GPS data
	if jsonGPSData.Longitude == 0 && jsonGPSData.Latitude == 0 && jsonGPSData.Altitude == 0 {
		return
	}

	// If altitude is below 0, set to 1 which means "below sea level"
	gpsData.AltitudeRef = 0
	if jsonGPSData.Altitude < 0 {
		gpsData.AltitudeRef = 1
	}

	gpsData.LatitudeRef = 'N'
	if jsonGPSData.Latitude < 0 {
		gpsData.LatitudeRef = 'S'
	}
	gpsData.LongitudeRef = 'E'
	if jsonGPSData.Longitude < 0 {
		gpsData.LongitudeRef = 'W'
	}

	gpsData.Latitude, gpsData.Longitude = jsonGPSData.DegreesToRational()
	gpsData.Altitude = []exifcommon.Rational{{Numerator: uint32(jsonGPSData.Altitude)}}
	gpsData.Timestamp = []exifcommon.Rational{
		{
			Numerator:   uint32(time.Time(jsonData.PhotoTakenTime.Timestamp).UTC().Hour()),
			Denominator: 1,
		},
		{
			Numerator:   uint32(time.Time(jsonData.PhotoTakenTime.Timestamp).UTC().Minute()),
			Denominator: 1,
		},
		{
			Numerator:   uint32(time.Time(jsonData.PhotoTakenTime.Timestamp).UTC().Second()),
			Denominator: 1,
		},
	}
	gpsData.Datestamp = time.Time(jsonData.PhotoTakenTime.Timestamp).UTC().Format("2006:01:02")

	gpsDegreesLatitude, err := exif.NewGpsDegreesFromRationals(string(gpsData.LatitudeRef), gpsData.Latitude)
	if err != nil {
		return
	}
	gpsDegreesLongitude, err := exif.NewGpsDegreesFromRationals(string(gpsData.LongitudeRef), gpsData.Longitude)
	if err != nil {
		return
	}

	x := time.Time(jsonData.PhotoTakenTime.Timestamp)
	y := time.Date(int(x.Year()), time.Month(x.Month()), int(x.Day()), x.Hour(), x.Minute(), x.Second(), 0, time.UTC)

	gpsData.GPSInfo = &exif.GpsInfo{
		Latitude:  gpsDegreesLatitude,
		Longitude: gpsDegreesLongitude,
		Timestamp: y,
	}

	return
}

// findAllJSONFilesInDir() function is used to find all JSON files in a specific directory
func findAllJSONFilesInDir(dir string) []string {
	var files []string
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			// Recurse into subdirectory
			subDir := filepath.Join(dir, fileInfo.Name())
			subFiles := findAllJSONFilesInDir(subDir)
			files = append(files, subFiles...)
		} else if strings.HasSuffix(fileInfo.Name(), ".json") {
			files = append(files, filepath.Join(dir, fileInfo.Name()))
		}
	}
	return files
}

// removeJSONFile() function is used to remove the JSON file after it has been processed
func removeJSONFile(jsonFile string) error {
	err := os.Remove(jsonFile)
	if err != nil {
		return err
	}

	return nil
}
