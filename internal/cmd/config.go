package cmd

import (
	"fmt"
	"os"

	"github.com/AdguardTeam/AdGuardDNS/internal/querylog"
	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/timeutil"
	"gopkg.in/yaml.v2"
)

// On-Disk Configuration File Entities
//
// These entities should only be used to parse and validate the on-disk
// configuration.  The order of the fields should generally not be altered.
//
// TODO(a.garipov): Consider collecting all validation errors instead of
// quitting after the first one.

// configuration represents the on-disk configuration of AdGuard DNS.
type configuration struct {
	// RateLimit is the rate limiting configuration.
	RateLimit *rateLimitConfig `yaml:"ratelimit"`

	// Cache is the DNS cache configuration.
	Cache *cacheConfig `yaml:"cache"`

	// Upstream is the configuration of upstream servers for the DNS servers.
	Upstream *upstreamConfig `yaml:"upstream"`

	// Backend is the AdGuard HTTP backend service configuration.  See the
	// environments type for more backend parameters.
	Backend *backendConfig `yaml:"backend"`

	// QueryLog is the additional query log configuration.  See the environments
	// type for more query log parameters.
	QueryLog *queryLogConfig `yaml:"query_log"`

	// GeoIP is the additional GeoIP database configuration.  See the
	// environments type for more GeoIP database parameters.
	GeoIP *geoIPConfig `yaml:"geoip"`

	// Check is the configuration for the DNS server checker.
	Check *checkConfig `yaml:"check"`

	// Web is the configuration for the DNS-over-HTTP server.
	Web *webConfig `yaml:"web"`

	// SafeBrowsing is the AdGuard general safe browsing filter configuration.
	SafeBrowsing *safeBrowsingConfig `yaml:"safe_browsing"`

	// AdultBlocking is the AdGuard adult content blocking filter configuration.
	AdultBlocking *safeBrowsingConfig `yaml:"adult_blocking"`

	// Filters contains the configuration for the filter lists and filtering
	// storage to be used.  They are used by filtering groups below.
	Filters *filtersConfig `yaml:"filters"`

	// ConnectivityCheck is the connectivity check configuration.
	ConnectivityCheck *connCheckConfig `yaml:"connectivity_check"`

	// AdditionalMetricsInfo is extra information, which is exposed by metrics.
	AdditionalMetricsInfo additionalInfo `yaml:"additional_metrics_info"`

	// FilteringGroups are the predefined filtering configurations that are used
	// for different server groups.
	FilteringGroups filteringGroups `yaml:"filtering_groups"`

	// ServerGroups are the DNS server groups.
	ServerGroups serverGroups `yaml:"server_groups"`
}

// errNilConfig signals that config is empty
const errNilConfig errors.Error = "nil config"

// buildQueryLog build an appropriate query log implementation from the
// configuration and environment data.  c is assumed to be valid.
func (c *configuration) buildQueryLog(envs *environments) (l querylog.Interface) {
	fileNeeded := c.QueryLog.File.Enabled
	if !fileNeeded {
		return querylog.Empty{}
	}

	return querylog.NewFileSystem(&querylog.FileSystemConfig{
		Path: envs.QueryLogPath,
	})
}

// validate returns an error if the configuration is invalid.
func (c *configuration) validate() (err error) {
	if c == nil {
		return errNilConfig
	}

	// Keep this in the same order as the fields in the config.
	validators := []struct {
		validate func() (err error)
		name     string
	}{{
		validate: c.RateLimit.validate,
		name:     "ratelimit",
	}, {
		validate: c.Upstream.validate,
		name:     "upstream",
	}, {
		validate: c.Cache.validate,
		name:     "cache",
	}, {
		validate: c.Backend.validate,
		name:     "backend",
	}, {
		validate: c.QueryLog.validate,
		name:     "query_log",
	}, {
		validate: c.GeoIP.validate,
		name:     "geoip",
	}, {
		validate: c.Check.validate,
		name:     "check",
	}, {
		validate: c.Web.validate,
		name:     "web",
	}, {
		validate: c.SafeBrowsing.validate,
		name:     "safe_browsing",
	}, {
		validate: c.AdultBlocking.validate,
		name:     "adult_blocking",
	}, {
		validate: c.Filters.validate,
		name:     "filters",
	}, {
		validate: c.FilteringGroups.validate,
		name:     "filtering groups",
	}, {
		validate: c.ServerGroups.validate,
		name:     "server_groups",
	}, {
		validate: c.ConnectivityCheck.validate,
		name:     "connectivity_check",
	}, {
		validate: c.AdditionalMetricsInfo.validate,
		name:     "additional_metrics_info",
	}}

	for _, v := range validators {
		err = v.validate()
		if err != nil {
			return fmt.Errorf("%s: %w", v.name, err)
		}
	}

	return nil
}

// queryLogConfig is the query log configuration.
type queryLogConfig struct {
	// File contains the JSONL file query log configuration.
	File *queryLogFileConfig `yaml:"file"`
}

// validate returns an error if the query log configuration is invalid.
func (c *queryLogConfig) validate() (err error) {
	switch {
	case c == nil:
		return errNilConfig
	case c.File == nil:
		return fmt.Errorf("file: %w", errNilConfig)
	default:
		return nil
	}
}

// queryLogFileConfig is the JSONL file query log configuration.
type queryLogFileConfig struct {
	Enabled bool `yaml:"enabled"`
}

// geoIPConfig is the GeoIP database configuration.
type geoIPConfig struct {
	// HostCacheSize is the size of the hostname lookup cache, in entries.
	HostCacheSize int `yaml:"host_cache_size"`

	// IPCacheSize is the size of the IP lookup cache, in entries.
	IPCacheSize int `yaml:"ip_cache_size"`

	// RefreshIvl defines how often AdGuard DNS reopens the GeoIP database
	// files.
	RefreshIvl timeutil.Duration `yaml:"refresh_interval"`
}

// validate returns an error if the GeoIP database configuration is invalid.
func (c *geoIPConfig) validate() (err error) {
	switch {
	case c == nil:
		return errNilConfig
	case c.HostCacheSize <= 0:
		// Note that while geoip.File can work with an empty host cache, that
		// feature is only used for tests.
		return newMustBePositiveError("host_cache_size", c.HostCacheSize)
	case c.IPCacheSize <= 0:
		return newMustBePositiveError("ip_cache_size", c.IPCacheSize)
	case c.RefreshIvl.Duration <= 0:
		return newMustBePositiveError("refresh_interval", c.RefreshIvl)
	default:
		return nil
	}
}

// readConfig reads the configuration.
func readConfig(confPath string) (c *configuration, err error) {
	// #nosec G304 -- Trust the path to the configuration file that is given
	// from the environment.
	yamlFile, err := os.ReadFile(confPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	c = &configuration{}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return c, nil
}
