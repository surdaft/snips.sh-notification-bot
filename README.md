# snips.sh-notification-bot

Send notifications when a new snip is posted.

Todo:

- [x] Use API to query snips
- [x] Keep history of posted snips, to avoid duplication (~~local sqlite? only needs to be small~~ using basic redis store)
- [x] Ensure age limit, to avoid new bots posting snips from a year ago (max lookback is 1hr)
- [x] Notification channels: email, webhook, discord, slack, webex, teams (~~is there a library for this already?~~ shoutrrr, just need to raise PR for webex)
