#!/bin/sh

contains() {
	key=$1
	shift
	for i in "$@"
	do
		if [[ $i == $key* ]]; then
			return 1
		fi
	done
	return 0
}

DEFAULT_NODE_ID=`hostname`
DEFAULT_ADV_ADDRESS=`hostname -f`

contains "-hostname" "$@"
if [ $? -eq 0 ]; then
	if [ -z "$HTTP_ADDR" ]; then
		HTTP_ADDR="0.0.0.0:4001"
	fi
	http_addr="-hostname $HTTP_ADDR"
fi

if [ -z "$DATA_DIR" ]; then
	DATA_DIR="/nyx/file/data"
fi

if [ -n "$KUBERNETES_SERVICE_HOST" ]; then
      if [ -z "$START_DELAY" ]; then
            START_DELAY=5
      fi
fi
if [ -n "$START_DELAY" ]; then
      sleep "$START_DELAY"
fi

NYX=/bin/nyx
nyx_commands="$NYX $http_addr"
data-dir="-data-dir $DATA_DIR"

if [ "$1" = "nyx" ]; then
        set -- $nyx_commands $data_dir
elif [ "${1:0:1}" = '-' ]; then
        # User is passing some options, so merge them.
        set -- $nyx_commands $@ $data_dir
fi

exec "$@"
