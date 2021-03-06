#!/bin/sh

# Always exit on errors
set -e

# Set known directories
SRL_ETC_DIR="/host/etc/opt/srlinux"
KUBEMGR_BIN_FILE="/kubemgr/bin/kubemgr"
KUBEMGR_YANG="/kubemgr/yang/kube.yang"
KUBEMGR_CONFIG="/kubemgr/kubemgr_config.yml"

# Give help text for parameters.
usage()
{
    printf "This is an entrypoint script for SR Linux Kubernetes Agent to overlay its\n"
    printf "binary and configuration into location in a filesystem.\n"
    printf "\n"
    printf "./entrypoint.sh\n"
    printf "\t-h --help\n"
    printf "\t--srl-etc-dir=%s\n" $SRL_ETC_DIR
    printf "\t--kubemgr-bin-file=%s\n" $KUBEMGR_BIN_FILE
    printf "\t--kubemgr-yang=%s\n" $KUBEMGR_YANG
    printf "\t--kubemgr-config=%s\n" $KUBEMGR_CONFIG
}

# Parse parameters given as arguments to this script.
while [ "$1" != "" ]; do
    PARAM=$(echo "$1" | awk -F= '{print $1}')
    VALUE=$(echo "$1" | awk -F= '{print $2}')
    case $PARAM in
        -h | --help)
            usage
            exit
            ;;
        --srl-etc-dir)
            SRL_ETC_DIR=$VALUE
            ;;
        --kubemgr-bin-file)
            KUBEMGR_BIN_FILE=$VALUE
            ;;
        --kubemgr-yang)
            KUBEMGR_YANG=$VALUE
            ;;
        --kubemgr-config)
            KUBEMGR_CONFIG=$VALUE
            ;;
        *)
            /bin/echo "ERROR: unknown parameter \"$PARAM\""
            usage
            exit 1
            ;;
    esac
    shift
done


# Loop through and verify each location each.
for i in $SRL_ETC_DIR $KUBEMGR_BIN_FILE $KUBEMGR_YANG $KUBEMGR_CONFIG
do
  if [ ! -e "$i" ]; then
    /bin/echo "Location $i does not exist"
    exit 1;
  fi
done

# Set up directories
mkdir -p ${SRL_ETC_DIR}/kubemgr/bin
mkdir -p ${SRL_ETC_DIR}/kubemgr/yang
mkdir -p ${SRL_ETC_DIR}/appmgr
# Add in the K8 service host/port, these are lost when app_mgr launches an application
# These env vars always exist when running inside K8
echo "Kubernetes service host is: $KUBERNETES_SERVICE_HOST"
echo "Kubernetes service port is: $KUBERNETES_SERVICE_PORT"
echo "Updating $KUBEMGR_CONFIG..."
sed -i 's/$KUBERNETES_SERVICE_HOST/'"$KUBERNETES_SERVICE_HOST"'/' "$KUBEMGR_CONFIG"
sed -i 's/$KUBERNETES_SERVICE_PORT/'"$KUBERNETES_SERVICE_PORT"'/' "$KUBEMGR_CONFIG"
sed -i 's/$KUBERNETES_NODE_NAME/'"$KUBERNETES_NODE_NAME"'/' "$KUBEMGR_CONFIG"
sed -i 's/$KUBERNETES_NODE_IP/'"$KUBERNETES_NODE_IP"'/' "$KUBEMGR_CONFIG"
# Copy files into proper places
cp -f "$KUBEMGR_BIN_FILE" "$SRL_ETC_DIR/kubemgr/bin/"
cp -f "$KUBEMGR_YANG" "$SRL_ETC_DIR/kubemgr/yang/"
cp -f "$KUBEMGR_CONFIG" "$SRL_ETC_DIR/appmgr/"
cp -f "/etc/config/config" "$SRL_ETC_DIR/config.json"

echo "Entering sleep... (success)"

# Sleep forever. 
# sleep infinity is not available in alpine; instead lets go sleep for ~68 years. Hopefully that's enough sleep
sleep 2147483647
