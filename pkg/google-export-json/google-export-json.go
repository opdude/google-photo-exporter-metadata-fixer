package googleexportjson

import (
	"math"
	"strconv"
	"strings"
	"time"

	exifcommon "github.com/dsoprea/go-exif/v3/common"
)

/*
	{
	  "title": "20221013_174131.jpg",
	  "description": "",
	  "imageViews": "0",
	  "creationTime": {
	    "timestamp": "1665676069",
	    "formatted": "13 Oct 2022, 15:47:49 UTC"
	  },
	  "photoTakenTime": {
	    "timestamp": "1665675691",
	    "formatted": "13 Oct 2022, 15:41:31 UTC"
	  },
	  "geoData": {
	    "latitude": 0.0,
	    "longitude": 0.0,
	    "altitude": 0.0,
	    "latitudeSpan": 0.0,
	    "longitudeSpan": 0.0
	  },
	  "geoDataExif": {
	    "latitude": 0.0,
	    "longitude": 0.0,
	    "altitude": 0.0,
	    "latitudeSpan": 0.0,
	    "longitudeSpan": 0.0
	  },
	  "url": "https://lh3.googleusercontent.com/myurl",
	  "googlePhotosOrigin": {
	    "mobileUpload": {
	      "deviceFolder": {
	        "localFolderName": ""
	      },
	      "deviceType": "ANDROID_PHONE"
	    }
	  }
	}
*/
type GoogleExportJSON struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	ImageViews         string `json:"imageViews"`
	CreationTime       `json:"creationTime"`
	PhotoTakenTime     `json:"photoTakenTime"`
	GeoData            `json:"geoData"`
	GeoDataExif        GeoData `json:"geoDataExif"`
	URL                string  `json:"url"`
	GooglePhotosOrigin `json:"googlePhotosOrigin"`
}

type CreationTime struct {
	Timestamp ParsedTime `json:"timestamp"`
	Formatted string     `json:"formatted"`
}

type PhotoTakenTime struct {
	Timestamp ParsedTime `json:"timestamp"`
	Formatted string     `json:"formatted"`
}

type GeoData struct {
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Altitude      float64 `json:"altitude"`
	LatitudeSpan  float64 `json:"latitudeSpan"`
	LongitudeSpan float64 `json:"longitudeSpan"`
}

type GooglePhotosOrigin struct {
	MobileUpload `json:"mobileUpload"`
}

type MobileUpload struct {
	DeviceFolder `json:"deviceFolder"`
	DeviceType   string `json:"deviceType"`
}

type DeviceFolder struct {
	LocalFolderName string `json:"localFolderName"`
}

type ParsedTime time.Time

func (p *ParsedTime) UnmarshalJSON(b []byte) error {
	value := strings.Trim(string(b), `"`) //get rid of "
	if value == "" || value == "null" {
		return nil
	}

	// convert the created timestamp st``ring to time.Time
	epoch, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return err
	}
	*p = ParsedTime(time.Unix(epoch, 0).UTC())

	return nil
}

func (p ParsedTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatInt(time.Time(p).Unix(), 10) + `"`), nil
}

func (g *GeoData) DegreesToRational() ([]exifcommon.Rational, []exifcommon.Rational) {
	return decimalToDMS(g.Latitude), decimalToDMS(g.Longitude)
}

func decimalToDMS(decimal float64) []exifcommon.Rational {
	decimal = math.Abs(decimal)
	degrees := int(decimal)
	decimal -= float64(degrees)
	decimal *= 60
	minutes := int(decimal)
	decimal -= float64(minutes)
	decimal *= 60
	seconds := int(decimal+0.5) * 100 // Round to nearest second

	return []exifcommon.Rational{
		{
			Numerator:   uint32(degrees),
			Denominator: 1,
		},
		{
			Numerator:   uint32(minutes),
			Denominator: 1,
		},
		{
			Numerator:   uint32(seconds),
			Denominator: 100,
		},
	}
}
