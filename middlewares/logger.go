package middlewares

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.Status = code
	r.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
	styles := map[string]string{
		"reset": "\033[0m",
		"bold":  "\033[1m",

		"blue":    "\033[34m", // GET
		"cyan":    "\033[36m", // POST
		"yellow":  "\033[33m", // PUT
		"magenta": "\033[35m", // DELETE
		"white":   "\033[37m",

		"green": "\033[32m", // 2xx
		"warn":  "\033[33m", // 4xx
		"red":   "\033[31m", // 5xx
		"gray":  "\033[90m",
	}

	methodColor := func(method string) string {
		switch method {
		case "GET":
			return styles["blue"]
		case "POST":
			return styles["cyan"]
		case "PUT":
			return styles["yellow"]
		case "DELETE":
			return styles["magenta"]
		default:
			return styles["white"]
		}
	}

	statusColor := func(code int) string {
		switch {
		case code >= 200 && code < 300:
			return styles["green"]
		case code >= 400 && code < 500:
			return styles["warn"]
		case code >= 500:
			return styles["red"]
		default:
			return styles["white"]
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, Status: http.StatusOK}
		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		method := fmt.Sprintf("%s%s%s%s", styles["bold"], methodColor(r.Method), r.Method, styles["reset"])
		status := fmt.Sprintf("%s%d%s", statusColor(rec.Status), rec.Status, styles["reset"])
		latency := fmt.Sprintf("%s%v%s", styles["gray"], duration, styles["reset"])
		path := r.URL.Path
		remote := fmt.Sprintf("%s%s%s", styles["bold"], r.RemoteAddr, styles["reset"])

		log.Printf("[%s] %s %s - %s from %s", method, status, path, latency, remote)
	})
}
