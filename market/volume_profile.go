package market

// VolumeProfileResponse holds the result of the VPVR calculation
type VolumeProfileResponse struct {
	POC            float64 // Point of Control (Price with max volume)
	VAHigh         float64 // Value Area High (70% vol)
	VALow          float64 // Value Area Low (70% vol)
	TotalVolume    float64
	ProfileBuckets []PriceBucket `json:"-"`
}

// PriceBucket represents a single price level bar in the histogram
type PriceBucket struct {
	PriceMid float64 // Midpoint of the bucket
	Volume   float64 // Total volume at this level
}

// calculateVPVR computes the Volume Profile visible Range for the given klines.
// It uses a fixed number of buckets (e.g., 100) across the high-low range.
func calculateVPVR(klines []Kline, buckets int) VolumeProfileResponse {
	if len(klines) == 0 {
		return VolumeProfileResponse{}
	}
	if buckets <= 0 {
		buckets = 24
	}

	// 1. Find Range (High/Low)
	minPrice := klines[0].Low
	maxPrice := klines[0].High
	totalVol := 0.0

	for _, k := range klines {
		if k.Low < minPrice {
			minPrice = k.Low
		}
		if k.High > maxPrice {
			maxPrice = k.High
		}
		totalVol += k.Volume
	}

	// Avoid division by zero
	if maxPrice <= minPrice || (maxPrice-minPrice) < 1e-8 {
		return VolumeProfileResponse{
			POC:         minPrice,
			VAHigh:      minPrice,
			VALow:       minPrice,
			TotalVolume: totalVol,
		}
	}

	// 2. Create Buckets
	rangeSize := maxPrice - minPrice
	bucketSize := rangeSize / float64(buckets)
	histogram := make([]float64, buckets)

	// 3. Populate Histogram
	// Logic: Distribute each candle's volume across the buckets it covers
	// Simplification: Assign all volume to the bucket of the Close price (or (H+L)/2)
	// For better accuracy: Weighted distribution. Let's use Midpoint ((H+L+C)/3) for now.
	for _, k := range klines {
		mid := (k.High + k.Low + k.Close) / 3.0
		// Find bucket index
		idx := int((mid - minPrice) / bucketSize)
		if idx >= buckets {
			idx = buckets - 1
		}
		if idx < 0 {
			idx = 0
		}
		histogram[idx] += k.Volume
	}

	// 4. Find POC (Point of Control)
	maxBucketVol := 0.0
	pocIndex := 0

	profileBuckets := make([]PriceBucket, buckets)

	for i, vol := range histogram {
		price := minPrice + (float64(i) * bucketSize) + (bucketSize / 2.0)
		profileBuckets[i] = PriceBucket{PriceMid: price, Volume: vol}

		if vol > maxBucketVol {
			maxBucketVol = vol
			pocIndex = i
		}
	}

	pocPrice := profileBuckets[pocIndex].PriceMid

	// 5. Calculate Value Area (70% of total volume)
	// Start from POC and expand outwards (up/down) until 70% threshold is reached.
	targetVol := totalVol * 0.70
	currentVol := maxBucketVol

	upIdx := pocIndex
	downIdx := pocIndex

	// Greedily expand to the side with more volume
	for currentVol < targetVol {
		nextUpVol := 0.0
		nextDownVol := 0.0

		// Check Up
		if upIdx+1 < buckets {
			nextUpVol = histogram[upIdx+1]
		}
		// Check Down
		if downIdx-1 >= 0 {
			nextDownVol = histogram[downIdx-1]
		}

		// Use floating point epsilon or simple logic
		// If both 0, we are done
		if nextUpVol == 0 && nextDownVol == 0 {
			break
		}

		if nextUpVol > nextDownVol {
			upIdx++
			currentVol += nextUpVol
		} else {
			downIdx--
			currentVol += nextDownVol
		}
	}

	vah := profileBuckets[upIdx].PriceMid
	val := profileBuckets[downIdx].PriceMid

	// Ensure logical sort (just in case loop logic was weird, though up/down should be correct)
	if val > vah {
		vah, val = val, vah
	}

	return VolumeProfileResponse{
		POC:            pocPrice,
		VAHigh:         vah,
		VALow:          val,
		TotalVolume:    totalVol,
		ProfileBuckets: profileBuckets,
	}
}
