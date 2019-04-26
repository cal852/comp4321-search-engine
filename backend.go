package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/dgraph-io/badger"

	"github.com/gorilla/mux"

	Indexer "github.com/davi1972/comp4321-search-engine/indexer"
)

type server struct {
	documentIndexer                   *Indexer.MappingIndexer
	wordIndexer                       *Indexer.MappingIndexer
	reverseDocumentIndexer            *Indexer.ReverseMappingIndexer
	reverseWordIndexer                *Indexer.ReverseMappingIndexer
	pagePropertiesIndexer             *Indexer.PagePropetiesIndexer
	titleInvertedIndexer              *Indexer.InvertedFileIndexer
	contentInvertedIndexer            *Indexer.InvertedFileIndexer
	documentWordForwardIndexer        *Indexer.DocumentWordForwardIndexer
	parentChildDocumentForwardIndexer *Indexer.ForwardIndexer
	childParentDocumentForwardIndexer *Indexer.ForwardIndexer
	router                            *mux.Router
}

type Edge struct {
	From int `json:"source"`
	To   int `json:"target"`
}

type EdgeString struct {
	From string `json:"source"`
	To   string `json:"target"`
}

type Node struct {
	Name string `json:"id"`
}

type GraphResponse struct {
	Nodes       []Node `json:"nodes"`
	Edges       []Edge `json:"links"`
	EdgesString []EdgeString
}

// S ...
var S server
var maxDepth = 2

func main() {
	S.Initialize()
	S.routes()
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		S.Release()
		os.Exit(1)
	}()
	S.parentChildDocumentForwardIndexer.Iterate()
	http.ListenAndServe("localhost:8000", S.router)
}

func (s *server) Initialize() {
	wd, _ := os.Getwd()
	s.documentIndexer = &Indexer.MappingIndexer{}
	docErr := s.documentIndexer.Initialize(wd + "/db/documentIndex")
	if docErr != nil {
		fmt.Printf("error when initializing document indexer: %s\n", docErr)
	}

	s.reverseDocumentIndexer = &Indexer.ReverseMappingIndexer{}
	reverseDocumentIndexerErr := s.reverseDocumentIndexer.Initialize(wd + "/db/reverseDocumentIndexer")
	if reverseDocumentIndexerErr != nil {
		fmt.Printf("error when initializing reverse document indexer: %s\n", reverseDocumentIndexerErr)
	}

	s.wordIndexer = &Indexer.MappingIndexer{}
	wordErr := s.wordIndexer.Initialize(wd + "/db/wordIndex")
	if wordErr != nil {
		fmt.Printf("error when initializing word indexer: %s\n", wordErr)
	}

	s.reverseWordIndexer = &Indexer.ReverseMappingIndexer{}
	reverseWordindexerErr := s.reverseWordIndexer.Initialize(wd + "/db/reverseWordIndexer")
	if reverseWordindexerErr != nil {
		fmt.Printf("error when initializing reverse word indexer: %s\n", reverseWordindexerErr)
	}

	s.pagePropertiesIndexer = &Indexer.PagePropetiesIndexer{}
	pagePropertiesErr := s.pagePropertiesIndexer.Initialize(wd + "/db/pagePropertiesIndex")
	if pagePropertiesErr != nil {
		fmt.Printf("error when initializing page properties: %s\n", pagePropertiesErr)
	}

	s.titleInvertedIndexer = &Indexer.InvertedFileIndexer{}
	titleInvertedErr := s.titleInvertedIndexer.Initialize(wd + "/db/titleInvertedIndex")
	if titleInvertedErr != nil {
		fmt.Printf("error when initializing page properties: %s\n", titleInvertedErr)
	}

	s.contentInvertedIndexer = &Indexer.InvertedFileIndexer{}
	contentInvertedErr := s.contentInvertedIndexer.Initialize(wd + "/db/contentInvertedIndex")
	if contentInvertedErr != nil {
		fmt.Printf("error when initializing page properties: %s\n", contentInvertedErr)
	}

	s.documentWordForwardIndexer = &Indexer.DocumentWordForwardIndexer{}
	documentWordForwardIndexerErr := s.documentWordForwardIndexer.Initialize(wd + "/db/documentWordForwardIndex")
	if documentWordForwardIndexerErr != nil {
		fmt.Printf("error when initializing document -> word forward Indexer: %s\n", documentWordForwardIndexerErr)
	}

	s.parentChildDocumentForwardIndexer = &Indexer.ForwardIndexer{}
	parentChildDocumentForwardIndexerErr := s.parentChildDocumentForwardIndexer.Initialize(wd + "/db/parentChildDocumentForwardIndex")
	if parentChildDocumentForwardIndexerErr != nil {
		fmt.Printf("error when initializing parentDocument -> childDocument forward Indexer: %s\n", parentChildDocumentForwardIndexerErr)
	}

	s.childParentDocumentForwardIndexer = &Indexer.ForwardIndexer{}
	childParentDocumentForwardIndexerErr := s.childParentDocumentForwardIndexer.Initialize(wd + "/db/childParentDocumentForwardIndex")
	if childParentDocumentForwardIndexerErr != nil {
		fmt.Printf("error when initializing childDocument -> parentDocument forward Indexer: %s\n", childParentDocumentForwardIndexerErr)
	}

	s.router = mux.NewRouter()

}

func (s *server) Release() {
	s.documentIndexer.Release()
	s.reverseDocumentIndexer.Release()
	s.wordIndexer.Release()
	s.reverseWordIndexer.Release()
	s.pagePropertiesIndexer.Release()
	s.titleInvertedIndexer.Release()
	s.contentInvertedIndexer.Release()
	s.documentWordForwardIndexer.Release()
	s.parentChildDocumentForwardIndexer.Release()
	s.childParentDocumentForwardIndexer.Release()
}

func (g *GraphResponse) AppendNodesAndEdgesStringFromIDList(docIDs []uint64) ([]uint64, error) {
	resultIDs := []uint64{}
	for _, docID := range docIDs {
		curStr, curErr := S.reverseDocumentIndexer.GetValueFromKey(docID)
		if curErr != nil {
			continue
		}
		idList, _ := S.parentChildDocumentForwardIndexer.GetIdListFromKey(uint64(docID))
		for _, i := range idList {
			str, valErr := S.reverseDocumentIndexer.GetValueFromKey(i)
			if valErr == badger.ErrKeyNotFound {
				continue
			} else if valErr != nil {
				return nil, valErr
			}
			g.Nodes = append(g.Nodes, Node{Name: str})
			g.EdgesString = append(g.EdgesString, EdgeString{From: curStr, To: str})
		}
		resultIDs = append(resultIDs, idList...)
	}

	return resultIDs, nil
}

// CreateEdgesID ...
func (g *GraphResponse) CreateEdgesID() {
	idMap := make(map[string]int)
	for i, val := range g.Nodes {
		if _, ok := idMap[val.Name]; !ok {
			idMap[val.Name] = i
		}
	}
	for _, val := range g.EdgesString {
		g.Edges = append(g.Edges, Edge{From: idMap[val.From], To: idMap[val.To]})
	}
}

func graphHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, convertErr := strconv.Atoi(vars["documentID"])
	if convertErr != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Invalid parameter value! Details: " + convertErr.Error()))
	}
	resp := &GraphResponse{}
	curIDList := []uint64{}
	var iterErr error
	// Append first id to curIDList
	curIDList = append(curIDList, uint64(id))
	for iterations := 0; iterations < maxDepth; iterations++ {
		curIDList, iterErr = resp.AppendNodesAndEdgesStringFromIDList(curIDList)
		if iterErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Internal Server Error! Details: " + iterErr.Error()))
			break
		}
	}
	resp.CreateEdgesID()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	jsonResult, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResult)
}

func (s *server) routes() {
	s.router.HandleFunc("/graph/{documentID}", graphHandler)
}