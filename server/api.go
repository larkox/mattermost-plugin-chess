package main

import (
	"fmt"
	"net/http"

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
	p.router.HandleFunc("/image.svg", p.handleImage).Methods(http.MethodGet)
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
				"(" + p.gameManager.GetBoardLink(gameID) + ")",
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

	post, err := p.gameManager.Move(gameID, userID, movement)
	if err != nil {
		interactiveDialogError(w, "Error: "+err.Error())
		return
	}

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

	post, err := p.gameManager.Resign(gameID, userID)
	if err != nil {
		interactiveDialogError(w, "Error: "+err.Error())
		return
	}

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
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		common.SlackAttachmentError(w, "Error: Not authorized")
		return
	}

	query := r.URL.Query()
	fen := query.Get("fen")
	if fen == "" {
		common.SlackAttachmentError(w, "Error: missing board definition")
		return
	}
	from := query.Get("from")
	to := query.Get("to")
	check := query.Get("check")
	capture := query.Get("capture")

	p.gameManager.PrintImage(w, fen, from, to, check, capture)
}
