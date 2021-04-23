package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-plugin-api/experimental/common"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (p *Plugin) initializeAPI() {
	p.router = mux.NewRouter()

	p.router.HandleFunc("/move/{id}", p.handleMove).Methods(http.MethodPost)
	p.router.HandleFunc("/movement/{id}", p.handleMovement).Methods(http.MethodPost)
	p.router.HandleFunc("/resign/{id}", p.handleResign).Methods(http.MethodPost)
	p.router.HandleFunc("/resignation/{id}", p.handleResignation).Methods(http.MethodPost)
	p.router.HandleFunc("/images/{id}", p.handleImage).Methods(http.MethodGet)
}

func (p *Plugin) handleMove(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		common.SlackAttachmentError(w, "Error: Not authorized")
		return
	}

	request := model.PostActionIntegrationRequestFromJson(r.Body)
	if request == nil {
		common.SlackAttachmentError(w, "Error: invalid request")
		return
	}

	if !p.gameManager.CanMove(gameID, userID) {
		common.SlackAttachmentError(w, "Cannot move.")
		return
	}

	baseURL := p.API.GetConfig().ServiceSettings.SiteURL
	appErr := p.API.OpenInteractiveDialog(model.OpenDialogRequest{
		TriggerId: request.TriggerId,
		URL:       fmt.Sprintf("%s/plugins/%s/movement/%s", *baseURL, manifest.Id, gameID),
		Dialog: model.Dialog{
			Title: "Make your move",
			IntroductionText: "Write your movement in default Algeabric Notation.\n\n![board]" +
				"(" + fmt.Sprintf("%s/plugins/%s/images/%s?ts=%s", *baseURL, manifest.Id, request.ChannelId, time.Now().Format("2006-01-02T15:04:05Z07:00")) + ")",
			SubmitLabel: "Move",
			Elements: []model.DialogElement{
				{
					DisplayName: "Movement",
					Name:        "movement",
					Type:        "text",
					HelpText:    "Ex. f3, Qh4",
				},
			},
		},
	})
	if appErr != nil {
		p.API.LogDebug("error opening move", "error", appErr.Error())
	}

	_, _ = w.Write((&model.PostActionIntegrationResponse{}).ToJson())
}

func (p *Plugin) handleResign(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		common.SlackAttachmentError(w, "Error: Not authorized")
		return
	}

	request := model.PostActionIntegrationRequestFromJson(r.Body)
	if request == nil {
		common.SlackAttachmentError(w, "Error: invalid request")
		return
	}

	if !p.gameManager.IsPlayingGame(gameID, userID) {
		common.SlackAttachmentError(w, "Error: you are not playing this game")
		return
	}

	baseURL := p.API.GetConfig().ServiceSettings.SiteURL
	appErr := p.API.OpenInteractiveDialog(model.OpenDialogRequest{
		TriggerId: request.TriggerId,
		URL:       fmt.Sprintf("%s/plugins/%s/resignation/%s", *baseURL, manifest.Id, gameID),
		Dialog: model.Dialog{
			Title:            "Resign this game?",
			IntroductionText: "Are you sure you want to resign this game?",
			SubmitLabel:      "Resign",
		},
	})

	if appErr != nil {
		common.SlackAttachmentError(w, "Error: could not open the interactive dialog, "+appErr.Error())
		return
	}

	_, _ = w.Write((&model.PostActionIntegrationResponse{}).ToJson())
}

func (p *Plugin) handleMovement(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		interactiveDialogError(w, "Error: Not authorized")
		return
	}

	request := model.SubmitDialogRequestFromJson(r.Body)
	if request == nil {
		interactiveDialogError(w, "Error: invalid request")
		return
	}

	movement := request.Submission["movement"].(string)

	err := p.gameManager.Move(gameID, userID, movement)
	if err != nil {
		interactiveDialogError(w, "Error: "+err.Error())
		return
	}

	post := p.gameManager.GetGamePost(gameID)
	_, _ = p.API.UpdatePost(post)

	_, _ = w.Write((&model.SubmitDialogResponse{}).ToJson())
}

func (p *Plugin) handleResignation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		interactiveDialogError(w, "Error: Not authorized")
		return
	}

	err := p.gameManager.Resign(gameID, userID)
	if err != nil {
		interactiveDialogError(w, "Error: "+err.Error())
		return
	}

	post := p.gameManager.GetGamePost(gameID)
	_, _ = p.API.UpdatePost(post)

	_, _ = w.Write((&model.SubmitDialogResponse{}).ToJson())
}

func interactiveDialogError(w http.ResponseWriter, message string) {
	resp := model.SubmitDialogResponse{
		Error: message,
	}

	_, _ = w.Write(resp.ToJson())
}

func (p *Plugin) handleImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		common.SlackAttachmentError(w, "Error: Not authorized")
		return
	}

	prefs, err := p.API.GetPreferencesForUser(userID)
	if err != nil {
		p.API.LogDebug("Could not get user preferences", "err", err)
	}

	light := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	dark := color.RGBA{R: 20, G: 93, B: 191, A: 255}
	highlight := color.RGBA{R: 255, G: 229, B: 119, A: 255}

	for _, pref := range prefs {
		if pref.Category == model.PREFERENCE_CATEGORY_THEME {
			type theme struct {
				SidebarBG          string `json:"sidebarBg"`
				MentionHighlightBG string `json:"mentionHighlightBg"`
				CenterChannelBG    string `json:"centerChannelBg"`
			}
			var t *theme
			err := json.Unmarshal([]byte(pref.Value), &t)
			if err != nil {
				p.API.LogDebug("Error unmarshaling values")
				break
			}
			if t == nil {
				p.API.LogDebug("Theme empty")
				break
			}

			newLight, err := ParseHexColor(t.CenterChannelBG)
			if err == nil {
				light = newLight
			} else {
				p.API.LogDebug("Error parsing", "err", err)
			}
			newDark, err := ParseHexColor(t.SidebarBG)
			if err == nil {
				dark = newDark
			}
			newHighlight, err := ParseHexColor(t.MentionHighlightBG)
			if err == nil {
				highlight = newHighlight
			}
			break
		}
	}

	p.gameManager.PrintImage(w, gameID, light, dark, highlight)
}

// Credit to: https://stackoverflow.com/questions/54197913/parse-hex-string-to-image-color
var errInvalidFormat = errors.New("invalid format")

func ParseHexColor(s string) (c color.RGBA, err error) {
	c.A = 0xff

	if s[0] != '#' {
		return c, errInvalidFormat
	}

	hexToByte := func(b byte) byte {
		switch {
		case b >= '0' && b <= '9':
			return b - '0'
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10
		}
		err = errInvalidFormat
		return 0
	}

	switch len(s) {
	case 7:
		c.R = hexToByte(s[1])<<4 + hexToByte(s[2])
		c.G = hexToByte(s[3])<<4 + hexToByte(s[4])
		c.B = hexToByte(s[5])<<4 + hexToByte(s[6])
	case 4:
		c.R = hexToByte(s[1]) * 17
		c.G = hexToByte(s[2]) * 17
		c.B = hexToByte(s[3]) * 17
	default:
		err = errInvalidFormat
	}
	return
}
}
