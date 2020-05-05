package api

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/trace/info"
	"github.com/DataDog/datadog-agent/pkg/util/log"
)

// profilingURLTemplate specifies the template for obtaining the profiling URL along with the site.
const profilingURLTemplate = "https://intake.profile.%s/v1/input"

// profileProxyHandler returns a new HTTP handler which will proxy requests to the profiling intake.
// If the URL can not be computed because of a malformed 'site' config, the returned handler will always
// return http.StatusInternalServerError along with a clarification.
func (r *HTTPReceiver) profileProxyHandler() http.Handler {
	target := fmt.Sprintf(profilingURLTemplate, config.Datadog.Get("site"))
	u, err := url.Parse(target)
	if err != nil {
		log.Errorf(`Invalid "site" provided: %v`, err)
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			msg := fmt.Sprintf(`Agent is misconfigured with an invalid "site". Not a valid target URL: %q`, target)
			http.Error(w, msg, http.StatusInternalServerError)
		})
	}
	return newProfileProxy(u, r.conf.APIKey())
}

// newProfileProxy creates a single-host reverse proxy with the given target, attaching
// the specified apiKey.
func newProfileProxy(target *url.URL, apiKey string) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		req.URL = target
		req.Header.Set("DD-API-KEY", apiKey)
		req.Header.Set("Via", fmt.Sprintf("trace-agent %s", info.Version))
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to the default value
			// that net/http gives it: Go-http-client/1.1
			// See https://codereview.appspot.com/7532043
			req.Header.Set("User-Agent", "")

		}
		containerID := req.Header.Get(headerContainerID)
		if tags := getContainerTags(containerID); tags != "" {
			req.Header.Set("X-Datadog-Container-Tags", tags)
		}
	}
	return &httputil.ReverseProxy{Director: director}
}
