//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package main

import (
	"encoding/json"
	"io"
	"net/http"

	bleveHttp "github.com/blevesearch/bleve/http"
	log "github.com/couchbaselabs/clog"
	"github.com/gorilla/mux"
)

func NewManagerRouter(mgr *Manager, staticDir string) (*mux.Router, error) {
	// create a router to serve static files
	router := staticFileRouter(staticDir)

	// add the API

	// these are custom handlers for cbft
	createIndexHandler := NewCreateIndexHander(mgr)
	router.Handle("/api/{indexName}", createIndexHandler).Methods("PUT")

	deleteIndexHandler := NewDeleteIndexHandler(mgr)
	router.Handle("/api/{indexName}", deleteIndexHandler).Methods("DELETE")

	// the rest are standard bleveHttp handlers
	getIndexHandler := bleveHttp.NewGetIndexHandler()
	router.Handle("/api/{indexName}", getIndexHandler).Methods("GET")

	listIndexesHandler := bleveHttp.NewListIndexesHander()
	router.Handle("/api", listIndexesHandler).Methods("GET")

	// docIndexHandler := bleveHttp.NewDocIndexHandler("")
	// router.Handle("/api/{indexName}/{docID}", docIndexHandler).Methods("PUT")

	docCountHandler := bleveHttp.NewDocCountHandler("")
	router.Handle("/api/{indexName}/_count", docCountHandler).Methods("GET")

	docGetHandler := bleveHttp.NewDocGetHandler("")
	router.Handle("/api/{indexName}/{docID}", docGetHandler).Methods("GET")

	// docDeleteHandler := bleveHttp.NewDocDeleteHandler("")
	// router.Handle("/api/{indexName}/{docID}", docDeleteHandler).Methods("DELETE")

	searchHandler := bleveHttp.NewSearchHandler("")
	router.Handle("/api/{indexName}/_search", searchHandler).Methods("POST")

	listFieldsHandler := bleveHttp.NewListFieldsHandler("")
	router.Handle("/api/{indexName}/_fields", listFieldsHandler).Methods("GET")

	debugHandler := bleveHttp.NewDebugDocumentHandler("")
	router.Handle("/api/{indexName}/{docID}/_debug", debugHandler).Methods("GET")

	return router, nil
}

func staticFileRouter(staticDir string) *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(true)

	// static
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
		myFileHandler{http.FileServer(http.Dir(staticDir))}))

	// application pages
	appPages := []string{
		"/overview",
		"/search",
		"/indexes",
		"/analysis",
		"/monitor",
	}

	for _, p := range appPages {
		// if you try to use index.html it will redirect...poorly
		r.PathPrefix(p).Handler(RewriteURL("/",
			http.FileServer(http.Dir(staticDir))))
	}

	r.Handle("/", http.RedirectHandler("/static/index.html", 302))

	return r
}

type myFileHandler struct {
	h http.Handler
}

func (mfh myFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if *staticEtag != "" {
		w.Header().Set("Etag", *staticEtag)
	}
	mfh.h.ServeHTTP(w, r)
}

func RewriteURL(to string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = to
		h.ServeHTTP(w, r)
	})
}

func showError(w http.ResponseWriter, r *http.Request,
	msg string, code int) {
	log.Printf("error: http: %v/%v", code, msg)
	http.Error(w, msg, code)
}

func mustEncode(w io.Writer, i interface{}) {
	if headered, ok := w.(http.ResponseWriter); ok {
		headered.Header().Set("Cache-Control", "no-cache")
		headered.Header().Set("Content-type", "application/json")
	}

	e := json.NewEncoder(w)
	if err := e.Encode(i); err != nil {
		panic(err)
	}
}