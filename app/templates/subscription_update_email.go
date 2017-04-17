package templates

import (
	"sort"
	"strings"
)

type UpdateSubscriptionEmailData struct {
	Playlist          *Playlist
	Activities        []*Activity
	ActorsDescription string
	AppBaseURL        string
}

func PrettyActorNames(activities []*Activity, max int) string {
	if len(activities) == 0 {
		return "Nobody"
	}

	var actorNames []string
	actorNameSet := make(map[string]bool)
	for _, activity := range activities {
		if _, ok := actorNameSet[activity.ActorName]; !ok {
			actorNameSet[activity.ActorName] = true
			actorNames = append(actorNames, activity.ActorName)
		}
	}

	sort.Strings(actorNames)

	if len(actorNames) > max {
		actorNames[max] = "and more"
		actorNames = actorNames[:max+1]
	}

	return strings.Join(actorNames, ", ")
}

var UpdateSubscriptionEmailHTML = parse("subscription_update_email")
