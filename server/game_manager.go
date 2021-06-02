package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"image/color"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strings"

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
	if game.Position().Turn() == chess.Black {
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

func (gm *GameManager) GetBoardLink(gameID string) string {
	game := gm.getGame(gameID)
	if game == nil {
		return ""
	}

	return gm.getBoardLink(game)
}

func (gm *GameManager) getBoardLink(game *chess.Game) string {
	baseURL := gm.api.GetConfig().ServiceSettings.SiteURL

	fen := game.FEN()
	movements := game.Moves()
	from := ""
	to := ""
	check := ""
	capture := ""
	if len(movements) > 0 {
		lastMovement := movements[len(movements)-1]
		from = lastMovement.S1().String()
		to = lastMovement.S2().String()
		if lastMovement.HasTag(chess.Check) {
			squareMap := game.Position().Board().SquareMap()
			for square, piece := range squareMap {
				if piece.Type() == chess.King && piece.Color() == squareMap[lastMovement.S2()].Color().Other() {
					check = square.String()
				}
			}
		}
		if lastMovement.HasTag(chess.Capture) {
			capture = lastMovement.S2().String()
			if lastMovement.HasTag(chess.EnPassant) {
				capture = lastMovement.S2().File().String() + lastMovement.S1().Rank().String()
			}
		}
	}

	imageURL, _ := url.Parse(fmt.Sprintf("%s/plugins/%s/image.svg", *baseURL, manifest.Id))
	q := imageURL.Query()
	q.Set("fen", fen)
	q.Set("from", from)
	q.Set("to", to)
	q.Set("check", check)
	q.Set("capture", capture)
	imageURL.RawQuery = q.Encode()
	return imageURL.String()
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
	if game.Position().Turn() == chess.Black {
		turn = "Black"
	}

	attachment := &model.SlackAttachment{
		Title:    "Chess game",
		ImageURL: gm.getBoardLink(game),
		Text:     fmt.Sprintf("White: %s\nBlack: %s", whiteUser.Username, blackUser.Username),
	}

	movements := game.Moves()
	check := false
	promoPiece := ""
	if len(movements) > 0 {
		lastMovement := movements[len(movements)-1]
		check = lastMovement.HasTag(chess.Check)
		if lastMovement.Promo() != chess.NoPieceType {
			promoPiece = pieceToPieceName[lastMovement.Promo()]
		}
	}

	if promoPiece != "" {
		attachment.Text += "\nPawn promoted to " + promoPiece
	}
	if check {
		attachment.Text += "\nCHECK!"
	}

	attachment.Text += "\nTurn: " + turn
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

func (gm *GameManager) PrintImage(w http.ResponseWriter, fen, from, to, check, capture string) {
	gf, err := chess.FEN(fen)
	if err != nil {
		return
	}
	g := chess.NewGame(gf)

	w.Header().Set("Content-Type", "image/svg+xml")
	buf := bytes.NewBuffer([]byte{})
	if from == "" {
		_ = chessImage.SVG(buf, g.Position().Board())
	} else {
		cian := color.RGBA{0, 255, 255, 1}
		red := color.RGBA{255, 0, 0, 1}

		redSquares := []chess.Square{}
		if capture != "" {
			redSquares = append(redSquares, strToSquareMap[capture])
		}
		if check != "" {
			redSquares = append(redSquares, strToSquareMap[check])
		}

		_ = chessImage.SVG(
			buf,
			g.Position().Board(),
			chessImage.MarkSquares(cian, strToSquareMap[from], strToSquareMap[to]),
			chessImage.MarkSquares(red, redSquares...),
		)
	}

	// Minor fixes for mobile strictness
	svgstring := string(buf.Bytes())
	// Add viewbox
	svgstrings := strings.Split(svgstring, "\n")
	r, _ := regexp.Compile("([a-z]+)=\"([0-9]+)\"")
	match := r.FindAllStringSubmatch(svgstrings[2], -1)
	svgstrings[2] += fmt.Sprintf(` viewBox="0 0 %s %s"`, match[0][2], match[1][2])
	svgstring = strings.Join(svgstrings, "\n")
	// Fix badly formed color fill
	r, _ = regexp.Compile(":000000")
	out := r.ReplaceAll([]byte(svgstring), []byte(":#000000"))
	w.Write([]byte(out))
}

var (
	pieceToPieceName = map[chess.PieceType]string{
		chess.King:   "King",
		chess.Queen:  "Queen",
		chess.Bishop: "Bishop",
		chess.Knight: "Knight",
		chess.Rook:   "Rook",
		chess.Pawn:   "Pawn",
	}
	strToSquareMap = map[string]chess.Square{
		"a1": chess.A1, "a2": chess.A2, "a3": chess.A3, "a4": chess.A4, "a5": chess.A5, "a6": chess.A6, "a7": chess.A7, "a8": chess.A8,
		"b1": chess.B1, "b2": chess.B2, "b3": chess.B3, "b4": chess.B4, "b5": chess.B5, "b6": chess.B6, "b7": chess.B7, "b8": chess.B8,
		"c1": chess.C1, "c2": chess.C2, "c3": chess.C3, "c4": chess.C4, "c5": chess.C5, "c6": chess.C6, "c7": chess.C7, "c8": chess.C8,
		"d1": chess.D1, "d2": chess.D2, "d3": chess.D3, "d4": chess.D4, "d5": chess.D5, "d6": chess.D6, "d7": chess.D7, "d8": chess.D8,
		"e1": chess.E1, "e2": chess.E2, "e3": chess.E3, "e4": chess.E4, "e5": chess.E5, "e6": chess.E6, "e7": chess.E7, "e8": chess.E8,
		"f1": chess.F1, "f2": chess.F2, "f3": chess.F3, "f4": chess.F4, "f5": chess.F5, "f6": chess.F6, "f7": chess.F7, "f8": chess.F8,
		"g1": chess.G1, "g2": chess.G2, "g3": chess.G3, "g4": chess.G4, "g5": chess.G5, "g6": chess.G6, "g7": chess.G7, "g8": chess.G8,
		"h1": chess.H1, "h2": chess.H2, "h3": chess.H3, "h4": chess.H4, "h5": chess.H5, "h6": chess.H6, "h7": chess.H7, "h8": chess.H8,
	}
)
