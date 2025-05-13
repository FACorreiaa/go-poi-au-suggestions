package types

type POIDetail struct {
	Name           string  `json:"name"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	Category       string  `json:"category"`
	DescriptionPOI string  `json:"description_poi"`
}
