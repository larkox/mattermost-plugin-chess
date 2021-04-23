package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"image/color"
	"math/big"
	"net/http"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/notnil/chess"
	chessImage "github.com/notnil/chess/image"
)

const (
	whiteTag   = "white"
	blackTag   = "black"
	channelTag = "channel"
	postTag    = "post"
)

type GameManager struct {
	api              plugin.API
	botID            string
	grantAchievement func(name string, userID string)
}

func NewGameManager(api plugin.API, botID string, grantAchievement func(name string, userID string)) GameManager {
	return GameManager{
		api:              api,
		botID:            botID,
		grantAchievement: grantAchievement,
	}
}

func (gm *GameManager) CreateGame(playerA, playerB string) error {
	c, appErr := gm.api.GetDirectChannel(playerA, playerB)
	if appErr != nil {
		return appErr
	}

	originalGame := gm.getGame(c.Id)
	if originalGame != nil {
		if originalGame.Outcome() == chess.NoOutcome {
			return errors.New("still an active game")
		}
	}

	game := chess.NewGame()
	r, _ := rand.Int(rand.Reader, big.NewInt(1))
	if r.Int64() == 0 {
		game.AddTagPair(whiteTag, playerA)
		game.AddTagPair(blackTag, playerB)
	} else {
		game.AddTagPair(whiteTag, playerB)
		game.AddTagPair(blackTag, playerA)
	}
	game.AddTagPair(channelTag, c.Id)

	gm.saveGame(game)

	post, _ := gm.api.CreatePost(gm.gameToPost(game))
	game.AddTagPair(postTag, post.Id)
	gm.saveGame(game)
	return nil
}

func (gm *GameManager) Move(id, player, movement string) error {
	game := gm.getGame(id)
	if game == nil {
		return errors.New("no game started")
	}

	_, _, whiteUser, blackUser := gm.getGameMetadata(game)

	turn := whiteUser
	if len(game.Moves())%2 == 1 {
		turn = blackUser
	}

	if player != turn.Id {
		return errors.New("it is not your turn")
	}

	err := game.MoveStr(movement)
	if err != nil {
		return err
	}

	gm.saveGame(game)
	return nil
}

func (gm *GameManager) Resign(id, player string) error {
	game := gm.getGame(id)
	if game == nil {
		return errors.New("no game started")
	}

	_, _, whiteUser, blackUser := gm.getGameMetadata(game)

	switch player {
	case whiteUser.Id:
		game.Resign(chess.White)
	case blackUser.Id:
		game.Resign(chess.Black)
	default:
		return errors.New("you are not playing")
	}

	gm.saveGame(game)
	return nil
}

func (gm *GameManager) getGame(id string) *chess.Game {
	b, appErr := gm.api.KVGet(id)
	if appErr != nil {
		return nil
	}

	pgn, err := chess.PGN(bytes.NewReader(b))
	if err != nil {
		return nil
	}

	return chess.NewGame(pgn)
}

func (gm *GameManager) saveGame(game *chess.Game) {
	id, _, _, _ := gm.getGameMetadata(game)
	_ = gm.api.KVSet(id, []byte(game.String()))
}

func (gm *GameManager) gameToPost(game *chess.Game) *model.Post {
	channelID, postID, whiteUser, blackUser := gm.getGameMetadata(game)

	baseURL := gm.api.GetConfig().ServiceSettings.SiteURL
	post := &model.Post{
		Id:        postID,
		ChannelId: channelID,
		UserId:    gm.botID,
	}

	turn := "White"
	if len(game.Moves())%2 == 1 {
		turn = "Black"
	}

	attachment := &model.SlackAttachment{
		Title:    "Chess game",
		ImageURL: fmt.Sprintf("%s/plugins/%s/images/%s?ts=%s", *baseURL, manifest.Id, channelID, time.Now().String()),
		Text:     fmt.Sprintf("White: %s\nBlack: %s\nTurn: %s", whiteUser.Username, blackUser.Username, turn),
	}

	switch game.Outcome() {
	case chess.NoOutcome:
		attachment.Actions = []*model.PostAction{
			{
				Type: "button",
				Name: "Move",
				Integration: &model.PostActionIntegration{
					URL: fmt.Sprintf("%s/plugins/%s/move/%s", *baseURL, manifest.Id, channelID),
				},
			},
			{
				Type: "button",
				Name: "Resign",
				Integration: &model.PostActionIntegration{
					URL: fmt.Sprintf("%s/plugins/%s/resign/%s", *baseURL, manifest.Id, channelID),
				},
			},
		}
	case chess.BlackWon:
		gm.grantAchievement(AchievementNameWinner, blackUser.Id)
		attachment.Footer = fmt.Sprintf("Black won by %s!", translateMethod(game.Method()))
	case chess.WhiteWon:
		gm.grantAchievement(AchievementNameWinner, whiteUser.Id)
		attachment.Footer = fmt.Sprintf("White won by %s!", translateMethod(game.Method()))
	case chess.Draw:
		attachment.Footer = fmt.Sprintf("Draw due to %s!", translateMethod(game.Method()))
	}

	model.ParseSlackAttachment(post, []*model.SlackAttachment{attachment})
	return post
}

func translateMethod(m chess.Method) string {
	switch m {
	case chess.Checkmate:
		return "Checkmate"
	case chess.DrawOffer:
		return "Draw offer"
	case chess.FiftyMoveRule:
		return "Fifty move rule"
	case chess.FivefoldRepetition:
		return "Fivefold Repetition"
	case chess.InsufficientMaterial:
		return "Insuficient Material"
	case chess.NoMethod:
		return "No method"
	case chess.Resignation:
		return "Resignation"
	case chess.SeventyFiveMoveRule:
		return "Seventy five move rule"
	case chess.Stalemate:
		return "Stalemate"
	case chess.ThreefoldRepetition:
		return "Threefold repetition"
	}
	return "Unknow method"
}

func (gm *GameManager) getGameMetadata(game *chess.Game) (string, string, *model.User, *model.User) {
	id := game.GetTagPair(channelTag).Value
	postID := ""
	whiteID := game.GetTagPair(whiteTag).Value
	blackID := game.GetTagPair(blackTag).Value

	postPair := game.GetTagPair(postTag)
	if postPair != nil {
		postID = postPair.Value
	}

	whiteUser, appErr := gm.api.GetUser(whiteID)
	if appErr != nil {
		return "", "", nil, nil
	}
	blackUser, appErr := gm.api.GetUser(blackID)
	if appErr != nil {
		return "", "", nil, nil
	}

	return id, postID, whiteUser, blackUser
}

func (gm *GameManager) GetGamePost(id string) *model.Post {
	g := gm.getGame(id)
	if g == nil {
		return nil
	}

	return gm.gameToPost(g)
}

func (gm *GameManager) CanMove(id, player string) bool {
	g := gm.getGame(id)
	if g == nil {
		return false
	}

	_, _, whitePlayer, blackPlayer := gm.getGameMetadata(g)

	turn := whitePlayer
	if len(g.Moves())%2 == 1 {
		turn = blackPlayer
	}

	return turn.Id == player
}

func (gm *GameManager) IsPlayingGame(id, player string) bool {
	g := gm.getGame(id)
	if g == nil {
		return false
	}

	_, _, whitePlayer, blackPlayer := gm.getGameMetadata(g)

	return whitePlayer.Id == player || blackPlayer.Id == player
}

func (gm *GameManager) PrintImage(w http.ResponseWriter, id string, light, dark, highlight color.RGBA) {
	g := gm.getGame(id)
	if g == nil {
		return
	}

	moves := g.Moves()
	w.Header().Set("Content-Type", "image/svg+xml")
	if len(moves) == 0 {
		_ = chessImage.SVG(w, g.Position().Board())
		return
	}

	lastMove := moves[len(moves)-1]
	_ = chessImage.SVG(
		w,
		g.Position().Board(),
		chessImage.MarkSquares(highlight, lastMove.S1(), lastMove.S2()),
		chessImage.SquareColors(light, dark),
	)
}
