package metrics

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"math"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"

	"git.sr.ht/~spc/go-log"

	"github.com/golang/snappy"
	"github.com/project-flotta/flotta-operator/models"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
)

const (
	defaultWaitInterval         = 5 * time.Minute
	defaultRequestNumSamples    = 10000
	defaultRequestDuration      = time.Hour
	defaultRequestRetryInterval = 10 * time.Second
	LastWriteFileName           = "metrics-lastwrite"
	DeviceLabel                 = "edgedeviceid"
)

type RemoteWrite struct {
	deviceID             string
	config               RemoteWriteConfiguration
	tsdb                 API
	LastWrite            time.Time
	lastWriteFile        string
	waitInterval         time.Duration
	RangeDuration        time.Duration
	RequestNumSamples    int
	RequestRetryInterval time.Duration
	Client               WriteClient
}

type RemoteWriteConfiguration struct {
	Url     string // empty URL means that feature is disabled
	Timeout time.Duration
}

func NewRemoteWrite(dataDir, deviceID string, tsdbInstance API) *RemoteWrite {
	waitInterval, _ := time.ParseDuration(os.Getenv("REMOTEWRITE_WAIT_INTERVAL"))
	if waitInterval == 0 {
		waitInterval = defaultWaitInterval
	}
	RangeDuration, _ := time.ParseDuration(os.Getenv("REMOTEWRITE_RANGE_DURATION"))
	if RangeDuration == 0 {
		RangeDuration = defaultRequestDuration
	}
	requestNumSamples, _ := strconv.Atoi(os.Getenv("REMOTEWRITE_REQUEST_NUM_SAMPLES"))
	if requestNumSamples == 0 {
		requestNumSamples = defaultRequestNumSamples
	}
	requestRetryInterval, _ := time.ParseDuration(os.Getenv("REMOTEWRITE_REQUEST_RETRY_INTERVAL"))
	if requestRetryInterval == 0 {
		requestRetryInterval = defaultRequestRetryInterval
	}

	newRemoteWrite := RemoteWrite{
		deviceID:             deviceID,
		tsdb:                 tsdbInstance,
		waitInterval:         waitInterval,
		RangeDuration:        RangeDuration,
		RequestNumSamples:    requestNumSamples,
		RequestRetryInterval: requestRetryInterval,
		lastWriteFile:        path.Join(dataDir, LastWriteFileName),
	}

	lastWriteBytes, err := ioutil.ReadFile(newRemoteWrite.lastWriteFile)
	if err == nil {
		if len(lastWriteBytes) != 0 {
			i, err := strconv.ParseInt(string(lastWriteBytes), 10, 64)
			if err == nil {
				newRemoteWrite.LastWrite = time.Unix(i/1000000000, i%1000000000)
			} else {
				log.Error(err)
			}
		}
	} else if !os.IsNotExist(err) {
		log.Error(err)
	}

	return &newRemoteWrite
}

// RemoteRecoverableError
// 'remote.RecoverableError' fields are private and can't be instantiated during testing. Therefore, we use this wrapper
type RemoteRecoverableError struct {
	error
}

func (e RemoteRecoverableError) Error() string {
	if e.error != nil {
		return e.Error()
	} else {
		return ""
	}
}

// WriteClient
// an interface used for testing
//go:generate mockgen -package=metrics -destination=write_client_mock.go . WriteClient
type WriteClient interface {
	Write(context.Context, []byte) error
}

type writeClient struct {
	client remote.WriteClient
}

func (w *writeClient) Write(ctx context.Context, data []byte) error {
	err := w.client.Store(ctx, data)
	switch err.(type) {
	case remote.RecoverableError:
		return RemoteRecoverableError{err}
	default:
		return err
	}
}

func (r *RemoteWrite) Update(config models.DeviceConfigurationMessage) error {
	// the following two variables are only here until we implement reading form device configuration
	serverURL := os.Getenv("REMOTEWRITE_SERVER_URL")
	serverTimeout, _ := time.ParseDuration(os.Getenv("REMOTEWRITE_SERVER_TIMEOUT"))
	if serverTimeout == 0 {
		serverTimeout = time.Second * 10
	}

	// create configuration
	r.config = RemoteWriteConfiguration{
		Url:     serverURL,
		Timeout: serverTimeout,
	}

	if r.config.Url == "" { // feature is disabled
		return nil
	}

	// create client
	if r.Client == nil {
		serverURL, err := url.Parse(r.config.Url)
		if err != nil {
			return err
		}
		client, err := remote.NewWriteClient(r.deviceID, &remote.ClientConfig{
			Timeout: model.Duration(r.config.Timeout),
			URL: &config_util.URL{
				URL: serverURL,
			},
		})
		if err != nil {
			return err
		}
		r.Client = &writeClient{
			client: client,
		}
	}

	return nil
}

func (r *RemoteWrite) Init(config models.DeviceConfigurationMessage) error {
	err := r.Update(config)
	go r.writeRoutine()
	return err
}

// writeRoutine
// Used as the goroutine function for handling ongoing writes to remote server
func (r *RemoteWrite) writeRoutine() {
	log.Infof("metric remote writer %s started", r.deviceID)
	for {
		r.Write()
		time.Sleep(r.waitInterval)
	}
}

// write
// Writes all the metrics. Stops when either no more metrics or failure (after retries)
func (r *RemoteWrite) Write() {

	if r.Client == nil { // feature disabled
		return
	}

	// start writing
	for maxTSDB := r.tsdb.MaxTime(); maxTSDB.Sub(r.LastWrite) > 0; maxTSDB = r.tsdb.MaxTime() {

		// set range start and end
		minTSDB, err := r.tsdb.MinTime()
		if err != nil {
			log.Error("failed reading TSDB min", err)
			return
		}

		rangeStart := r.LastWrite.Add(time.Millisecond) // TSDB time is in milliseconds
		if rangeStart.Sub(minTSDB) < 0 {
			rangeStart = minTSDB
		}
		rangeEnd := rangeStart.Add(r.RangeDuration)
		if rangeEnd.Sub(maxTSDB) > 0 {
			rangeEnd = maxTSDB
		}

		log.Infof("going to write metrics range %s(%d)-%s(%d). TSDB min max: %s(%d)-%s(%d)",
			rangeStart.String(), rangeStart.UnixNano(), rangeEnd.String(), rangeEnd.UnixNano(),
			minTSDB.String(), minTSDB.UnixNano(), maxTSDB.String(), maxTSDB.UnixNano(),
		)

		// read range
		series, err := r.tsdb.GetMetricsForTimeRange(rangeStart, rangeEnd, true)
		if err != nil {
			if err != nil {
				log.Errorf("failed reading metrics for range %d-%d", rangeStart.UnixNano(), rangeEnd.UnixNano())
				return
			}
		}

		// add device label
		for _, s := range series {
			s.Labels[DeviceLabel] = r.deviceID
		}

		// write range
		if len(series) != 0 {
			if r.writeRange(series) {
				log.Infof("wrote metrics range %s(%d)-%s(%d)", rangeStart.String(), rangeStart.UnixNano(), rangeEnd.String(), rangeEnd.UnixNano())
			} else {
				return // all tries failed. we will retry later
			}
		} else {
			log.Info("metrics range is empty")
		}

		// store last write
		r.LastWrite = rangeEnd
		err = ioutil.WriteFile(r.lastWriteFile, strconv.AppendInt(nil, r.LastWrite.UnixNano(), 10), 0600)
		if err != nil {
			log.Error(err)
		}
	}
}

// return value indicates whether or not to continue to next range
// not necessarily that everything was written and successful
func (r *RemoteWrite) writeRange(series []Series) bool {
	requestSeries := make([]Series, 0, len(series)) // the series slice for current request
	sampleIndex := 0                                // location within current series
	numSamples := 0                                 // how many samples we accumulated for the request

	for i := 0; i < len(series); {
		currentSeries := series[i]
		lenCurrentSeries := len(currentSeries.DataPoints)

		if lenCurrentSeries != 0 {
			if sampleIndex == 0 || len(requestSeries) == 0 { // first time we're using this series or requestSeries was reset
				requestSeries = append(requestSeries, Series{Labels: currentSeries.Labels})
			}

			numMissingSamples := r.RequestNumSamples - numSamples
			remainderOfSeries := lenCurrentSeries - sampleIndex
			numSamplesToUse := numMissingSamples
			if numMissingSamples > remainderOfSeries {
				numSamplesToUse = remainderOfSeries
			}

			samplesToAppend := currentSeries.DataPoints[sampleIndex : sampleIndex+numSamplesToUse]
			numSamples += numSamplesToUse
			sampleIndex += numSamplesToUse

			currReqSeries := &(requestSeries[len(requestSeries)-1])
			currReqSeries.DataPoints = append(currReqSeries.DataPoints, samplesToAppend...)
		}

		if sampleIndex == lenCurrentSeries {
			// advance to next series
			i++
			sampleIndex = 0
		}

		// if request not full and this is not the last series then continue to next series
		if numSamples != r.RequestNumSamples && i < len(series) {
			continue
		}

		if !r.writeRequest(requestSeries) {
			return false
		}

		numSamples = 0
		requestSeries = requestSeries[:0]
	}

	return true
}

// return value indicates whether or not to continue to next request
// not necessarily that everything was written and successful
func (r *RemoteWrite) writeRequest(series []Series) bool {
	timeSeries, lowest, highest := toTimeSeries(series)

	writeRequest := prompb.WriteRequest{
		Timeseries: timeSeries,
	}

	if log.CurrentLevel() >= log.LevelTrace {
		reqBytes, err := json.Marshal(writeRequest)
		if err != nil {
			log.Error("can not marshal prompb.WriteRequest to json", err)
		}
		log.Tracef("sending write request (lowest %s highest %s): %s", lowest.String(), highest.String(), string(reqBytes))
	}

	reqBytes, err := writeRequest.Marshal()
	if err != nil {
		log.Error("can not marshal prompb.WriteRequest", err)
		return false
	}

	reqBytes = snappy.Encode(nil, reqBytes)

	const numTries = 3
	for try := 1; try <= numTries; try++ {
		err = r.Client.Write(context.TODO(), reqBytes)
		if err == nil {
			break
		}

		log.Errorf("sending metrics remote write request failed. try No.%d. error: %s", try, err.Error())

		switch err.(type) {
		case RemoteRecoverableError:
			if try == numTries {
				log.Error("aborting metrics remote write request due to too many tries")
				return false
			} else {
				time.Sleep(r.RequestRetryInterval)
			}
		default:
			try = numTries
			// cause loop to break. skip this request cause error not recoverable
			// e.g. already wrote these samples
		}
	}

	return true
}

// toTimeSeries
// converst to TimeSeries and returns lowest and highest timestamps
func toTimeSeries(series []Series) (timeSeries []prompb.TimeSeries, lowest time.Time, highest time.Time) {
	mint := int64(math.MaxInt64)
	maxt := int64(0)

	timeSeries = make([]prompb.TimeSeries, len(series))

	for i, s := range series {
		ts := &(timeSeries[i])
		ts.Labels = make([]prompb.Label, 0, len(s.Labels))
		for k, v := range s.Labels {
			ts.Labels = append(ts.Labels, prompb.Label{
				Name:  k,
				Value: v,
			})
		}
		ts.Samples = make([]prompb.Sample, 0, len(s.DataPoints))
		for _, dp := range s.DataPoints {
			ts.Samples = append(ts.Samples, prompb.Sample{
				Value:     dp.Value,
				Timestamp: dp.Time,
			})
			if dp.Time < mint {
				mint = dp.Time
			}
			if dp.Time > maxt {
				maxt = dp.Time
			}
		}
	}

	lowest = fromDbTime(mint)
	highest = fromDbTime(maxt)
	return
}
