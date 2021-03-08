package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

func getHelp() string {
	return `Available Commands:

challenge @user
	Challenge a user for a game of chess
`
}

func getCommand() *model.Command {
	return &model.Command{
		Trigger:          "chess",
		DisplayName:      "Chess Bot",
		Description:      "Play chess",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: challenge",
		AutoCompleteHint: "[command]",
		AutocompleteData: getAutocompleteData(),
	}
}

func (p *Plugin) postCommandResponse(args *model.CommandArgs, text string) {
	post := &model.Post{
		UserId:    p.BotUserID,
		ChannelId: args.ChannelId,
		Message:   text,
	}
	_ = p.API.SendEphemeralPost(args.UserId, post)
}

// ExecuteCommand executes a given command and returns a command response.
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	spaceRegExp := regexp.MustCompile(`\s+`)
	trimmedArgs := spaceRegExp.ReplaceAllString(strings.TrimSpace(args.Command), " ")
	stringArgs := strings.Split(trimmedArgs, " ")
	lengthOfArgs := len(stringArgs)
	restOfArgs := []string{}

	var handler func([]string, *model.CommandArgs) (bool, *model.CommandResponse, error)
	if lengthOfArgs == 1 {
		p.postCommandResponse(args, getHelp())
		return &model.CommandResponse{}, nil
	}
	command := stringArgs[1]
	if lengthOfArgs > 2 {
		restOfArgs = stringArgs[2:]
	}
	switch command {
	case "challenge":
		handler = p.runChallengeCommand
	default:
		p.postCommandResponse(args, getHelp())
		return &model.CommandResponse{}, nil
	}
	isUserError, resp, err := handler(restOfArgs, args)
	if err != nil {
		if isUserError {
			p.postCommandResponse(args, fmt.Sprintf("__Error: %s.__\n\nRun `/todo help` for usage instructions.", err.Error()))
		} else {
			p.API.LogError(err.Error())
			p.postCommandResponse(args, "An unknown error occurred. Please talk to your system administrator for help.")
		}
	}

	if resp != nil {
		return resp, nil
	}

	return &model.CommandResponse{}, nil
}

func (p *Plugin) runChallengeCommand(args []string, extra *model.CommandArgs) (bool, *model.CommandResponse, error) {
	if len(args) < 1 {
		p.postCommandResponse(extra, "You must specify a user to challenge.\n"+getHelp())
		return false, nil, nil
	}

	userName := args[0]
	if args[0][0] == '@' {
		userName = args[0][1:]
	}
	receiver, appErr := p.API.GetUserByUsername(userName)
	if appErr != nil {
		p.postCommandResponse(extra, "Please, provide a valid user.\n"+getHelp())
		return false, nil, nil
	}

	if receiver.Id == extra.UserId {
		p.postCommandResponse(extra, "You cannot challenge yourself.\n"+getHelp())
		return false, nil, nil
	}

	err := p.gameManager.CreateGame(extra.UserId, receiver.Id)
	if err != nil {
		p.postCommandResponse(extra, "Could not create the game. Error: "+err.Error())
		return false, nil, nil
	}

	t, appErr := p.API.GetTeam(extra.TeamId)
	if appErr != nil {
		p.postCommandResponse(extra, "Game created, but could not redirect you to the DM. Error: "+appErr.Error())
		return false, nil, nil
	}

	// Navigate to DM
	return false, &model.CommandResponse{
		GotoLocation: extra.SiteURL + "/" + t.Name + "/messages/@" + receiver.Username,
	}, nil
}

func getAutocompleteData() *model.AutocompleteData {
	chess := model.NewAutocompleteData("chess", "[command]", "Available commands: challenge")

	challenge := model.NewAutocompleteData("challenge", "[user]", "Challenges a user")
	challenge.AddTextArgument("Whom to challenge", "[@someone]", "")
	chess.AddCommand(challenge)

	return chess
}
