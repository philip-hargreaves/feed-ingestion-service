package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

func NewRegistry(deps Dependencies) *Registry {
	if deps.ExecutablePath == nil {
		deps.ExecutablePath = os.Executable
	}

	reg := &Registry{
		state: &state{
			cfg:            deps.Config,
			db:             deps.Store,
			fetchFeed:      deps.FetchFeed,
			executablePath: deps.ExecutablePath,
		},
		cmds: &commands{
			handlers: make(map[string]func(*state, command) error),
		},
	}

	reg.cmds.register("help", handlerHelp)
	reg.cmds.register("login", handlerLogin)
	reg.cmds.register("register", handlerRegister)
	reg.cmds.register("users", handlerUsers)
	reg.cmds.register("agg", handlerAgg)
	reg.cmds.register("supervise", handlerSupervise)
	reg.cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	reg.cmds.register("follow", middlewareLoggedIn(handlerFollow))
	reg.cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	reg.cmds.register("following", middlewareLoggedIn(handlerFollowing))
	reg.cmds.register("browse", middlewareLoggedIn(handlerBrowse))
	reg.cmds.register("feeds", handlerFeeds)
	reg.cmds.register("reset", handlerReset)

	return reg
}

func (r *Registry) Run(ctx context.Context, cmd Command) error {
	_ = ctx
	return r.cmds.run(r.state, command{
		name: cmd.Name,
		args: cmd.Args,
	})
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.cfg.GetCurrentUserName())
		if err != nil {
			return fmt.Errorf("Couldn't get current user: %w", err)
		}
		return handler(s, cmd, user)
	}
}
