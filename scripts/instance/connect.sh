#!/bin/bash
# Connect to the persistent Zellij session.
# Called by ttyd for each browser connection.
#
# `attach --create` attaches to an existing session named "main", or creates
# a new one if it doesn't exist. The default layout (set in Zellij config)
# is used when creating.
#
# When the browser disconnects, only the Zellij client exits; the server and
# all running processes (including Claude) continue working in the background.

exec zellij attach --create main
