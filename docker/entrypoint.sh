#!/bin/bash
set -e

# Delegate all process management to the supervisor
exec /supervisor.sh
