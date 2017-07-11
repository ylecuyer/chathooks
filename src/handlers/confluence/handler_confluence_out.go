package confluence

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"

	cc "github.com/commonchat/commonchat-go"
	"github.com/grokify/webhookproxy/src/adapters"
	"github.com/grokify/webhookproxy/src/config"
	"github.com/grokify/webhookproxy/src/util"
	"github.com/valyala/fasthttp"
)

const (
	DisplayName      = "Confluence"
	HandlerKey       = "confluence"
	MessageDirection = "out"
)

// FastHttp request handler for Confluence outbound webhook
// https://developer.atlassian.com/static/connect/docs/beta/modules/common/webhook.html
type Handler struct {
	Config  config.Configuration
	Adapter adapters.Adapter
}

// FastHttp request handler constructor for Confluence outbound webhook
func NewHandler(cfg config.Configuration, adapter adapters.Adapter) Handler {
	return Handler{Config: cfg, Adapter: adapter}
}

func (h Handler) HandlerKey() string {
	return HandlerKey
}

func (h Handler) MessageDirection() string {
	return MessageDirection
}

// HandleFastHTTP is the method to respond to a fasthttp request.
func (h *Handler) HandleFastHTTP(ctx *fasthttp.RequestCtx) {
	ccMsg, err := Normalize(h.Config, ctx.FormValue("payload"))

	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusNotAcceptable)
		log.WithFields(log.Fields{
			"type":   "http.response",
			"status": fasthttp.StatusNotAcceptable,
		}).Info(fmt.Sprintf("%v request is not acceptable.", DisplayName))
		return
	}

	util.SendWebhook(ctx, h.Adapter, ccMsg)
}

func Normalize(cfg config.Configuration, bytes []byte) (cc.Message, error) {
	ccMsg := cc.NewMessage()
	iconURL, err := cfg.GetAppIconURL(HandlerKey)
	if err == nil {
		ccMsg.IconURL = iconURL.String()
	}

	src, err := ConfluenceOutMessageFromBytes(bytes)
	if err != nil {
		return ccMsg, err
	}

	if !src.IsComment() {
		if src.Page.IsCreated() {
			ccMsg.Activity = fmt.Sprintf("%v created page", src.Page.CreatorName)
		} else {
			ccMsg.Activity = fmt.Sprintf("%v updated page", src.Page.CreatorName)
		}
	} else {
		if src.Comment.IsCreated() {
			ccMsg.Activity = fmt.Sprintf("%v commented on page", src.Comment.CreatorName)
		} else {
			ccMsg.Activity = fmt.Sprintf("%v updated comment on page", src.Comment.CreatorName)
		}
	}

	attachment := cc.NewAttachment()

	if len(src.Page.Title) > 0 && len(src.Page.Self) > 0 {
		attachment.AddField(cc.Field{
			Title: "Page",
			Value: fmt.Sprintf("[%v](%v)", src.Page.Title, src.Page.Self),
			Short: true})
	}
	if len(src.Page.SpaceKey) > 0 {
		field := cc.Field{Title: "Space", Short: true}
		if src.IsComment() {
			field.Value = src.Comment.Parent.SpaceKey
		} else {
			field.Value = src.Page.SpaceKey
		}
		attachment.AddField(field)
	}

	ccMsg.AddAttachment(attachment)
	return ccMsg, nil
}

type ConfluenceOutMessage struct {
	User      string            `json:"user,omitempty"`
	UserKey   string            `json:"userKey,omitempty"`
	Timestamp int64             `json:"timestamp,omitempty"`
	Username  string            `json:"username,omitempty"`
	Page      ConfluencePage    `json:"page,omitempty"`
	Comment   ConfluenceComment `json:"comment,omitempty"`
}

func ConfluenceOutMessageFromBytes(bytes []byte) (ConfluenceOutMessage, error) {
	log.WithFields(log.Fields{
		"type":    "message.raw",
		"message": string(bytes),
	}).Debug(fmt.Sprintf("%v message.", DisplayName))
	msg := ConfluenceOutMessage{}
	err := json.Unmarshal(bytes, &msg)
	if err != nil {
		log.WithFields(log.Fields{
			"type":  "message.json.unmarshal",
			"error": fmt.Sprintf("%v\n", err),
		}).Warn(fmt.Sprintf("%v request unmarshal failure.", DisplayName))
	}
	if msg.IsComment() {
		msg.Page = msg.Comment.Parent
	}
	return msg, err
}

func (msg *ConfluenceOutMessage) IsComment() bool {
	if msg.Comment.ModificationDate > 0 {
		return true
	}
	return false
}

type ConfluencePage struct {
	SpaceKey         string `json:"spaceKey,omitempty"`
	ModificationDate int64  `json:"modificationDate,omitempty"`
	CreatorKey       string `json:"creatorKey,omitempty"`
	CreatorName      string `json:"creatorName,omitempty"`
	LastModifierKey  string `json:"lastModifierKey,omitempty"`
	Self             string `json:"self,omitempty"`
	LastModifierName string `json:"lastModifierName,omitempty"`
	Id               int64  `json:"id,omitempty"`
	Title            string `json:"title,omitempty"`
	CreationDate     int64  `json:"creationDate,omitempty"`
	Version          int64  `json:"version,omitempty"`
}

func (page *ConfluencePage) IsCreated() bool {
	if page.ModificationDate > 0 && page.ModificationDate == page.CreationDate {
		return true
	}
	return false
}

func (page *ConfluencePage) IsUpdated() bool {
	if page.IsCreated() {
		return false
	}
	return true
}

type ConfluenceComment struct {
	SpaceKey         string         `json:"spaceKey,omitempty"`
	Parent           ConfluencePage `json:"parent,omitempty"`
	ModificationDate int64          `json:"modificationDate,omitempty"`
	CreatorKey       string         `json:"creatorKey,omitempty"`
	CreatorName      string         `json:"creatorName,omitempty"`
	LastModifierKey  string         `json:"lastModifierKey,omitempty"`
	Self             string         `json:"self,omitempty"`
	LastModifierName string         `json:"lastModifierName,omitempty"`
	Id               int64          `json:"id,omitempty"`
	CreationDate     int64          `json:"creationDate,omitempty"`
	Version          int64          `json:"version,omitempty"`
}

func (comment *ConfluenceComment) IsCreated() bool {
	if comment.ModificationDate > 0 && comment.ModificationDate == comment.CreationDate {
		return true
	}
	return false
}

func (comment *ConfluenceComment) IsUpdated() bool {
	if comment.IsCreated() {
		return false
	}
	return true
}
