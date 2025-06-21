package poi

import "fmt"

func getGeneralPOIByDistance(lat, lon, distance float64) string {
	return fmt.Sprintf(`
            Generate a list of points of interest that people usually see no matter. Could be points of interest, bars, restaurants, hotels, activities, etc.
            The user location is at latitude %0.2f and longitude %0.2f.
            Only include points of interest that are within %0.2f kilometers from the user's location.
            Return the response STRICTLY as a JSON object with:
            {
            "points_of_interest": [
                {
                "name": "Name of the Point of Interest",
                "latitude": <float>,
                "longitude": <float>,
                "category": "Primary category (e.g., Museum, Historical Site, Park, Restaurant, Bar)",
                "description_poi": "A 2-3 sentence description of this specific POI and why it's relevant."
                }
            ]
            }`, lat, lon, distance/1000)
}
