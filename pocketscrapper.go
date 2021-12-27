package pocketscrapper

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/alesr/pocketcli"
	"github.com/cixtor/readability"
	"github.com/google/uuid"
)

const defaultCtxTimeout = time.Duration(10 * time.Second)

type (
	Scrapper struct {
		httpCli *http.Client
		cleaner *readability.Readability
	}

	Item struct {
		originID   int
		id         string
		title      string
		url        string
		rawContent []byte
		article    *readability.Article
	}
)

func New(httpCli *http.Client) *Scrapper {
	return &Scrapper{
		httpCli: httpCli,
		cleaner: readability.New(),
	}
}

func (s *Scrapper) Scrap(ctx context.Context, bookmarks map[string]pocketcli.Bookmark) []*Item {
	items := parsePocketBookmarks(bookmarks)

	itemChan := make(chan *Item)
	errChan := make(chan error)

	for _, item := range items {
		go s.process(ctx, item, itemChan, errChan)
	}

	scrappedItems := make([]*Item, 0)

	for i := 0; i < len(items); i++ {
		select {
		case item := <-itemChan:
			scrappedItems = append(scrappedItems, item)
		case err := <-errChan:
			log.Println(fmt.Errorf("could not scrap item: %s", err))
		}
	}
	return scrappedItems
}

func (s *Scrapper) process(ctx context.Context, item *Item, itemChan chan *Item, errChan chan error) {
	fetchCtx, cancel := context.WithTimeout(ctx, defaultCtxTimeout)
	defer cancel()

	content, err := s.fetchPageContent(fetchCtx, item.url)
	if err != nil {
		errChan <- fmt.Errorf("could not fetch page content for item '%s': %s", item.id, err)
		return
	}
	item.rawContent = content

	article, err := s.extractReadableContent(content, item.url)
	if err != nil {
		errChan <- fmt.Errorf("could not extract article for item '%s': %s", item.id, err)
		return
	}
	item.article = article
	itemChan <- item
}

func (s *Scrapper) fetchPageContent(ctx context.Context, url string) ([]byte, error) {
	resp, err := s.httpCli.Get(url)
	if err != nil {
		return nil, fmt.Errorf("could not fetch page: %s", err)
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response body: %s", err)
	}
	return content, nil
}

func (s *Scrapper) extractReadableContent(rawContent []byte, url string) (*readability.Article, error) {
	article, err := s.cleaner.Parse(bytes.NewBuffer(rawContent), url)
	if err != nil {
		return nil, fmt.Errorf("could not parse article: %s", err)
	}
	return &article, nil
}

func parsePocketBookmarks(bookmarks map[string]pocketcli.Bookmark) []*Item {
	items := make([]*Item, 0, len(bookmarks))
	for _, b := range bookmarks {
		items = append(items, &Item{
			id:       uuid.NewString(),
			originID: b.ID,
			title:    b.Title,
			url:      b.URL,
		})
	}
	return items
}
