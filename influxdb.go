/*
	InfluxDB client
	(c) Copyright David Thorpe 2017
	All Rights Reserved

	For Licensing and Usage information, please see LICENSE file
*/
package influxpi

import (
	"errors"
	"fmt"
	"time"

	gopi "github.com/djthorpe/gopi"
	client "github.com/influxdata/influxdb/client/v2"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Config defines the configuration parameters for connecting to Influx Database
type Config struct {
	Host     string
	Port     uint
	SSL      bool
	Database string
	Username string
	Password string
	Timeout  time.Duration
}

// Client defines a connection to an Influx Database
type Client struct {
	log      gopi.Logger
	database string
	addr     string
	config   client.HTTPConfig
	client   client.Client
	version  string
}

////////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	// DefaultPortHTTP defines the default InfluxDB port used for HTTP
	DefaultPortHTTP uint = 8086
)

////////////////////////////////////////////////////////////////////////////////
// GLOBAL VARIABLES

var (
	// ErrNotConnected is returned when database is not connected (Close has been called)
	ErrNotConnected = errors.New("No connection")

	// ErrUnexpectedResponse is returned when server does not return with expected data
	ErrUnexpectedResponse = errors.New("Unexpected response from server")

	// ErrEmptyResponse is returned when response does not contain any data
	ErrEmptyResponse = errors.New("Empty response from server")
)

////////////////////////////////////////////////////////////////////////////////
// OPEN AND CLOSE

// Open returns an InfluxDB client object
func (config Config) Open(log gopi.Logger) (gopi.Driver, error) {
	log.Debug2("<influxdb.Client>Open{ addr=%v database=%v }", config.addr(), config.Database)

	this := new(Client)
	this.log = log
	this.addr = config.addr()
	this.config = client.HTTPConfig{
		Addr:     this.addr,
		Username: config.Username,
		Password: config.Password,
		Timeout:  config.Timeout,
	}

	var err error
	if this.client, err = client.NewHTTPClient(this.config); err != nil {
		return nil, this.log.Error("%v", err)
	}

	// Ping client to make sure it exists, get InfluxDB version
	var t time.Duration
	if t, this.version, err = this.client.Ping(this.config.Timeout); err != nil {
		this.client.Close()
		this.client = nil
		return nil, this.log.Error("%v", err)
	}
	this.log.Debug("InfluxDB Version=%v Ping=%v", this.version, t)

	// Set database
	if config.Database != "" {
		if err := this.SetDatabase(config.Database); err != nil {
			return nil, this.log.Error("Unknown database: %v", config.Database)
		}
	}

	// Return success
	return this, nil
}

// Close releases any resources associated with the client connection
func (this *Client) Close() error {
	this.log.Debug2("<influxdb.Client>Close")
	if this.client != nil {
		if err := this.client.Close(); err != nil {
			this.client = nil
			return err
		}
		this.client = nil
	}
	return nil
}

func (config Config) addr() string {
	method := "http"
	if config.SSL {
		method = "https"
	}
	if config.Port == 0 {
		config.Port = DefaultPortHTTP
	}
	return fmt.Sprintf("%v://%v:%v/", method, config.Host, config.Port)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// GetVersion returns the version string for the InfluxDB
func (this *Client) GetVersion() string {
	if this.client == nil {
		return ""
	} else {
		return this.version
	}
}

// GetDatabase returns the current database
func (this *Client) GetDatabase() string {
	if this.client == nil {
		return ""
	} else {
		return this.database
	}
}

// SetDatabase sets the current database to use, will
// return ErrEmptyResponse if the database doesn't exist
func (this *Client) SetDatabase(name string) error {
	if this.client == nil {
		return ErrNotConnected
	}
	if databases, err := this.ShowDatabases(); err != nil {
		return err
	} else {
		for _, value := range databases {
			if value == name {
				this.database = value
				return nil
			}
		}
	}
	return ErrEmptyResponse
}

// ShowDatabases enumerates the databases
func (this *Client) ShowDatabases() ([]string, error) {
	if this.client == nil {
		return nil, ErrNotConnected
	}
	if values, err := this.queryScalar("SHOW DATABASES", "databases", "name"); err != nil {
		return nil, err
	} else {
		return values, nil
	}
}

// GetMeasurements enumerates the measurements for a database
func (this *Client) GetMeasurements() ([]string, error) {
	if this.client == nil {
		return nil, ErrNotConnected
	}
	if values, err := this.queryScalar("SHOW MEASUREMENTS", "measurements", "name"); err != nil {
		return nil, err
	} else {
		return values, nil
	}
}

// Create Database with an optional retention policy
// CREATE DATABASE <database_name> [WITH [DURATION <duration>] [REPLICATION <n>] [SHARD DURATION <duration>] [NAME <retention-policy-name>]]
func (this *Client) CreateDatabase(name string, policy *RetentionPolicy) error {
	if this.client == nil {
		return ErrNotConnected
	}
	q := "CREATE DATABASE \"" + name + "\""
	this.log.Debug2(q)
	return ErrNotConnected
}

// Delete Database
func (this *Client) DropDatabase(name string) error {
	if this.client == nil {
		return ErrNotConnected
	}
	return ErrNotConnected
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (this *Client) String() string {
	if this.client != nil {
		return fmt.Sprintf("influxdb.Client{ connected=true addr=%v%v version=%v }", this.addr, this.database, this.GetVersion())
	} else {
		return fmt.Sprintf("influxdb.Client{ connected=false addr=%v%v }", this.addr, this.database)
	}
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Query database and return response or error
func (this *Client) query(query string) (*client.Response, error) {
	if this.client == nil {
		return nil, ErrNotConnected
	}
	response, err := this.client.Query(client.Query{
		Command:  query,
		Database: this.database,
	})
	if err != nil {
		return nil, err
	}
	if response.Error() != nil {
		return nil, response.Error()
	}
	return response, nil
}

// queryTable returns a table structure
func (this *Client) Query(query string) (*Table, error) {
	// Query and sanity check the response
	response, err := this.query(query)
	if err != nil {
		return nil, err
	}
	if len(response.Results) != 1 {
		return nil, ErrEmptyResponse
	}
	if response.Results[0].Series == nil || len(response.Results[0].Series) == 0 {
		return nil, ErrEmptyResponse
	}

	// Copy model.Row over to Table structure
	series := response.Results[0].Series[0]
	table := new(Table)
	table.Name = series.Name
	table.Tags = series.Tags
	table.Columns = series.Columns
	table.Values = series.Values
	table.Partial = series.Partial
	return table, nil
}

// queryScalar returns a single column of string values
func (this *Client) queryScalar(query, dataset, column string) ([]string, error) {
	// Query and sanity check the response
	table, err := this.Query(query)
	if err != nil {
		return nil, err
	}
	// Sanity check the data returned
	if table.Name != dataset {
		return nil, ErrUnexpectedResponse
	}
	if len(table.Columns) != 1 && table.Columns[0] != column {
		return nil, ErrUnexpectedResponse
	}

	values := make([]string, 0, len(table.Values))
	for _, column := range table.Values {
		if len(column) != 1 {
			return nil, ErrUnexpectedResponse
		}
		if value, ok := column[0].(string); ok == false {
			return nil, ErrUnexpectedResponse
		} else {
			values = append(values, value)
		}

	}

	return values, nil
}