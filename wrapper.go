package goinsta

import (
	"errors"
	"net/http"
	"time"
)

const TooManyRequestsTimeout = 8 * time.Minute

type ReqWrapper interface {
	GoInstaWrapper(*ReqWrapperArgs) (body []byte, h http.Header, err error)
}

type ReqWrapperArgs struct {
	insta      *Instagram
	reqOptions *reqOptions

	Body    []byte
	Headers http.Header
	Error   error
}

func (w *ReqWrapperArgs) RetryRequest() ([]byte, http.Header, error) {
	return w.insta.sendRequest(w.reqOptions)
}

func (w *ReqWrapperArgs) GetWrapperCount() int      { return w.reqOptions.WrapperCount }
func (w *ReqWrapperArgs) GetInsta() *Instagram      { return w.insta }
func (w *ReqWrapperArgs) GetEndpoint() string       { return w.reqOptions.Endpoint }
func (w *ReqWrapperArgs) SetInsta(insta *Instagram) { w.insta = insta }
func (w *ReqWrapperArgs) Ignore429() bool           { return w.reqOptions.Ignore429 }

type Wrapper struct{}

func DefaultWrapper() *Wrapper { return &Wrapper{} }

func (w *Wrapper) GoInstaWrapper(o *ReqWrapperArgs) ([]byte, http.Header, error) {
	if o.Error == nil || o.GetWrapperCount() > 3 {
		return o.Body, o.Headers, o.Error
	}
	if errors.Is(o.Error, ErrTooManyRequests) {
		if o.Ignore429() {
			return o.Body, o.Headers, o.Error
		}
		o.GetInsta().warnHandler("Too many requests, sleeping for", TooManyRequestsTimeout)
		time.Sleep(TooManyRequestsTimeout)
		return o.RetryRequest()
	}
	return o.Body, o.Headers, o.Error
}
