PODKOP_LIB="/usr/lib/podkop"
. "$PODKOP_LIB/helpers.sh"
. "$PODKOP_LIB/sing_box_config_manager.sh"

sing_box_cf_add_dns_server() {
    local config="$1"
    local type="$2"
    local tag="$3"
    local server="$4"
    local domain_resolver="$5"
    local detour="$6"

    local server_address server_port
    server_address=$(url_get_host "$server")
    server_port=$(url_get_port "$server")

    case "$type" in
    udp)
        [ -z "$server_port" ] && server_port=53
        config=$(sing_box_cm_add_udp_dns_server "$config" "$tag" "$server_address" "$server_port" "$domain_resolver" \
            "$detour")
        ;;
    dot)
        [ -z "$server_port" ] && server_port=853
        config=$(sing_box_cm_add_tls_dns_server "$config" "$tag" "$server_address" "$server_port" "$domain_resolver" \
            "$detour")
        ;;
    doh)
        [ -z "$server_port" ] && server_port=443
        local path headers
        path=$(url_get_path "$server")
        headers="" # TODO(ampetelin): implement it if necessary
        config=$(sing_box_cm_add_https_dns_server "$config" "$tag" "$server_address" "$server_port" "$path" "$headers" \
            "$domain_resolver" "$detour")
        ;;
    *)
        log "Unsupported DNS server type: $type. Aborted." "fatal"
        exit 1
        ;;
    esac

    echo "$config"
}

sing_box_cf_add_mixed_inbound_and_route_rule() {
    local config="$1"
    local tag="$2"
    local listen_address="$3"
    local listen_port="$4"
    local outbound="$5"

    config=$(sing_box_cm_add_mixed_inbound "$config" "$tag" "$listen_address" "$listen_port")
    config=$(sing_box_cm_add_route_rule "$config" "" "$tag" "$outbound")

    echo "$config"
}

sing_box_cf_add_proxy_outbound() {
    local config="$1"
    local section="$2"
    local url="$3"
    local udp_over_tcp="$4"

    url=$(url_decode "$url")
    url=$(url_strip_fragment "$url")

    local scheme
    scheme="$(url_get_scheme "$url")"
    case "$scheme" in
    socks4 | socks4a | socks5)
        local tag host port version userinfo username password udp_over_tcp

        tag=$(get_outbound_tag_by_section "$section")
        host=$(url_get_host "$url")
        port=$(url_get_port "$url")
        version="${scheme#socks}"
        if [ "$scheme" = "socks5" ]; then
            userinfo=$(url_get_userinfo "$url")
            if [ -n "$userinfo" ]; then
                username="${userinfo%%:*}"
                password="${userinfo#*:}"
            fi
        fi
        config="$(sing_box_cm_add_socks_outbound \
            "$config" \
            "$tag" \
            "$host" \
            "$port" \
            "$version" \
            "$username" \
            "$password" \
            "" \
            "$([ "$udp_over_tcp" == "1" ] && echo 2)" # if udp_over_tcp is enabled, enable version 2
        )"
        ;;
    vless)
        local tag host port uuid flow packet_encoding
        tag=$(get_outbound_tag_by_section "$section")
        host=$(url_get_host "$url")
        port=$(url_get_port "$url")
        uuid=$(url_get_userinfo "$url")
        flow=$(url_get_query_param "$url" "flow")
        packet_encoding=$(url_get_query_param "$url" "packetEncoding")

        config=$(sing_box_cm_add_vless_outbound "$config" "$tag" "$host" "$port" "$uuid" "$flow" "" "$packet_encoding")
        config=$(_add_outbound_security "$config" "$tag" "$url")
        config=$(_add_outbound_transport "$config" "$tag" "$url")
        ;;
    ss)
        local userinfo tag host port method password udp_over_tcp

        userinfo=$(url_get_userinfo "$url")
        if ! is_shadowsocks_userinfo_format "$userinfo"; then
            userinfo=$(base64_decode "$userinfo")
            if [ $? -ne 0 ]; then
                log "Cannot decode shadowsocks userinfo or it does not match the expected format. Aborted." "fatal"
                exit 1
            fi
        fi

        tag=$(get_outbound_tag_by_section "$section")
        host=$(url_get_host "$url")
        port=$(url_get_port "$url")
        method="${userinfo%%:*}"
        password="${userinfo#*:}"

        config=$(
            sing_box_cm_add_shadowsocks_outbound \
                "$config" \
                "$tag" \
                "$host" \
                "$port" \
                "$method" \
                "$password" \
                "" \
                "$([ "$udp_over_tcp" == "1" ] && echo 2)" # if udp_over_tcp is enabled, enable version 2
        )
        ;;
    trojan)
        local tag host port password
        tag=$(get_outbound_tag_by_section "$section")
        host=$(url_get_host "$url")
        port=$(url_get_port "$url")
        password=$(url_get_userinfo "$url")

        config=$(sing_box_cm_add_trojan_outbound "$config" "$tag" "$host" "$port" "$password")
        config=$(_add_outbound_security "$config" "$tag" "$url")
        config=$(_add_outbound_transport "$config" "$tag" "$url")
        ;;
    hysteria2 | hy2)
        local tag host port password obfuscator_type obfuscator_password upload_mbps download_mbps
        tag=$(get_outbound_tag_by_section "$section")
        host=$(url_get_host "$url")
        port="$(url_get_port "$url")"
        password=$(url_get_userinfo "$url")
        obfuscator_type=$(url_get_query_param "$url" "obfs")
        obfuscator_password=$(url_get_query_param "$url" "obfs-password")
        upload_mbps=$(url_get_query_param "$url" "upmbps")
        download_mbps=$(url_get_query_param "$url" "downmbps")

        config=$(sing_box_cm_add_hysteria2_outbound "$config" "$tag" "$host" "$port" "$password" "$obfuscator_type" \
            "$obfuscator_password" "$upload_mbps" "$download_mbps")
        config=$(_add_outbound_security "$config" "$tag" "$url")
        ;;
    vpn)
        if ! command -v python3 >/dev/null 2>&1; then
            log "vpn:// URIs require python3-light (opkg install python3-light). Aborted." "fatal"
            exit 1
        fi
        local decoded_url
        decoded_url="$(python3 /usr/lib/podkop/amnezia_vpn_uri_to_vless.py "$url")" || exit $?
        config="$(sing_box_cf_add_proxy_outbound "$config" "$section" "$decoded_url" "$udp_over_tcp")"
        ;;
    awg2)
        config="$(sing_box_cf_add_awg2_interface_outbound "$config" "$section" "$url")"
        ;;
    *)
        log "Unsupported proxy $scheme type. Aborted." "fatal"
        exit 1
        ;;
    esac

    echo "$config"
}

_awg_cfg_write_if_set() {
    local key="$1"
    local value="$2"
    local cfg_file="$3"
    if [ -n "$value" ]; then
        echo "${key} = ${value}" >> "$cfg_file"
    fi
}

_setup_awg2_interface_from_uri() {
    local section="$1"
    local uri="$2"

    local host port address private_key mtu public_key preshared_key allowed_ips persistent_keepalive \
        jc jmin jmax s1 s2 s3 s4 h1 h2 h3 h4 i1 i2 i3 i4 i5

    host="$(url_get_host "$uri")"
    port="$(url_get_port "$uri")"
    address="$(url_get_query_param "$uri" "address")"
    private_key="$(url_get_query_param "$uri" "private_key")"
    mtu="$(url_get_query_param "$uri" "mtu")"
    public_key="$(url_get_query_param "$uri" "public_key")"
    preshared_key="$(url_get_query_param "$uri" "preshared_key")"
    allowed_ips="$(url_get_query_param "$uri" "allowed_ips")"
    persistent_keepalive="$(url_get_query_param "$uri" "persistent_keepalive")"
    jc="$(url_get_query_param "$uri" "jc")"
    jmin="$(url_get_query_param "$uri" "jmin")"
    jmax="$(url_get_query_param "$uri" "jmax")"
    s1="$(url_get_query_param "$uri" "s1")"
    s2="$(url_get_query_param "$uri" "s2")"
    s3="$(url_get_query_param "$uri" "s3")"
    s4="$(url_get_query_param "$uri" "s4")"
    h1="$(url_get_query_param "$uri" "h1")"
    h2="$(url_get_query_param "$uri" "h2")"
    h3="$(url_get_query_param "$uri" "h3")"
    h4="$(url_get_query_param "$uri" "h4")"
    i1="$(url_get_query_param "$uri" "i1")"
    i2="$(url_get_query_param "$uri" "i2")"
    i3="$(url_get_query_param "$uri" "i3")"
    i4="$(url_get_query_param "$uri" "i4")"
    i5="$(url_get_query_param "$uri" "i5")"

    if [ -z "$host" ] || [ -z "$port" ] || [ -z "$private_key" ] || [ -z "$public_key" ]; then
        log "Invalid awg2:// URI: missing host/port/private_key/public_key. Aborted." "fatal"
        exit 1
    fi

    [ -z "$address" ] && address="10.255.255.2/32"
    [ -z "$allowed_ips" ] && allowed_ips="0.0.0.0/0,::/0"
    [ -z "$persistent_keepalive" ] && persistent_keepalive="25"

    local ifname hash cfg_file
    hash="$(printf '%s' "$section" | md5sum | cut -c1-8)"
    ifname="pawg${hash}"

    ip link del dev "$ifname" 2>/dev/null || true
    ip link add dev "$ifname" type amneziawg || {
        log "Failed to create amneziawg interface '$ifname'. Aborted." "fatal"
        exit 1
    }
    [ -n "$mtu" ] && ip link set mtu "$mtu" dev "$ifname" || true
    ip address flush dev "$ifname" 2>/dev/null || true
    ip address add "$address" dev "$ifname"

    cfg_file="$(mktemp /tmp/podkop-awg2.XXXXXX)"
    chmod 600 "$cfg_file"
    {
        echo "[Interface]"
        echo "PrivateKey = $private_key"
    } > "$cfg_file"
    _awg_cfg_write_if_set "Jc" "$jc" "$cfg_file"
    _awg_cfg_write_if_set "Jmin" "$jmin" "$cfg_file"
    _awg_cfg_write_if_set "Jmax" "$jmax" "$cfg_file"
    _awg_cfg_write_if_set "S1" "$s1" "$cfg_file"
    _awg_cfg_write_if_set "S2" "$s2" "$cfg_file"
    _awg_cfg_write_if_set "S3" "$s3" "$cfg_file"
    _awg_cfg_write_if_set "S4" "$s4" "$cfg_file"
    _awg_cfg_write_if_set "H1" "$h1" "$cfg_file"
    _awg_cfg_write_if_set "H2" "$h2" "$cfg_file"
    _awg_cfg_write_if_set "H3" "$h3" "$cfg_file"
    _awg_cfg_write_if_set "H4" "$h4" "$cfg_file"
    _awg_cfg_write_if_set "I1" "$i1" "$cfg_file"
    _awg_cfg_write_if_set "I2" "$i2" "$cfg_file"
    _awg_cfg_write_if_set "I3" "$i3" "$cfg_file"
    _awg_cfg_write_if_set "I4" "$i4" "$cfg_file"
    _awg_cfg_write_if_set "I5" "$i5" "$cfg_file"
    {
        echo ""
        echo "[Peer]"
        echo "PublicKey = $public_key"
        [ -n "$preshared_key" ] && echo "PresharedKey = $preshared_key"
        IFS=','; for ip_cidr in $allowed_ips; do
            ip_cidr="$(echo "$ip_cidr" | tr -d ' ')"
            [ -n "$ip_cidr" ] && echo "AllowedIPs = $ip_cidr"
        done; unset IFS
        echo "Endpoint = ${host}:${port}"
        echo "PersistentKeepalive = ${persistent_keepalive}"
    } >> "$cfg_file"

    awg setconf "$ifname" "$cfg_file" || {
        rm -f "$cfg_file"
        log "Failed to apply awg2 config to '$ifname'. Aborted." "fatal"
        exit 1
    }
    rm -f "$cfg_file"
    ip link set up dev "$ifname"

    echo "$ifname"
}

sing_box_cf_add_awg2_interface_outbound() {
    local config="$1"
    local section="$2"
    local awg2_uri="$3"

    local tag ifname
    tag="$(get_outbound_tag_by_section "$section")"
    ifname="$(_setup_awg2_interface_from_uri "$section" "$awg2_uri")"
    config="$(sing_box_cm_add_interface_outbound "$config" "$tag" "$ifname")"
    echo "$config"
}

_add_outbound_security() {
    local config="$1"
    local outbound_tag="$2"
    local url="$3"

    local security scheme
    security=$(url_get_query_param "$url" "security")
    if [ -z "$security" ]; then
        scheme="$(url_get_scheme "$url")"
        if [ "$scheme" = "hysteria2" ] || [ "$scheme" = "hy2" ]; then
            security="tls"
        fi
    fi

    case "$security" in
    tls | reality)
        local sni insecure alpn fingerprint public_key short_id
        sni=$(url_get_query_param "$url" "sni")
        insecure=$(_get_insecure_query_param_from_url "$url")
        alpn=$(comma_string_to_json_array "$(url_get_query_param "$url" "alpn")")
        fingerprint=$(url_get_query_param "$url" "fp")
        public_key=$(url_get_query_param "$url" "pbk")
        short_id=$(url_get_query_param "$url" "sid")

        config=$(
            sing_box_cm_set_tls_for_outbound \
                "$config" \
                "$outbound_tag" \
                "$sni" \
                "$([ "$insecure" == "1" ] && echo true)" \
                "$([ "$alpn" == "[]" ] && echo null || echo "$alpn")" \
                "$fingerprint" \
                "$public_key" \
                "$short_id"
        )
        ;;
    none) ;;
    *)
        log "Unknown security '$security' detected." "error"
        ;;
    esac

    echo "$config"
}

_get_insecure_query_param_from_url() {
    local url="$1"

    local insecure
    insecure=$(url_get_query_param "$url" "allowInsecure")
    if [ -z "$insecure" ]; then
        insecure=$(url_get_query_param "$url" "insecure")
    fi

    echo "$insecure"
}

_add_outbound_transport() {
    local config="$1"
    local outbound_tag="$2"
    local url="$3"

    local transport
    transport=$(url_get_query_param "$url" "type")
    case "$transport" in
    tcp | raw) ;;
    ws)
        local ws_path ws_host ws_early_data
        ws_path=$(url_get_query_param "$url" "path")
        ws_host=$(url_get_query_param "$url" "host")
        ws_early_data=$(url_get_query_param "$url" "ed")

        config=$(
            sing_box_cm_set_ws_transport_for_outbound "$config" "$outbound_tag" "$ws_path" "$ws_host" "$ws_early_data"
        )
        ;;
    grpc)
        # TODO(ampetelin): Add handling of optional gRPC parameters; example links are needed.
        local grpc_service_name
        grpc_service_name=$(url_get_query_param "$url" "serviceName")

        config=$(
            sing_box_cm_set_grpc_transport_for_outbound "$config" "$outbound_tag" "$grpc_service_name"
        )
        ;;
    *)
        log "Unknown transport '$transport' detected." "error"
        ;;
    esac

    echo "$config"
}

sing_box_cf_add_json_outbound() {
    local config="$1"
    local section="$2"
    local json_outbound="$3"

    local tag
    tag=$(get_outbound_tag_by_section "$section")

    config=$(sing_box_cm_add_raw_outbound "$config" "$tag" "$json_outbound")

    echo "$config"
}

sing_box_cf_add_interface_outbound() {
    local config="$1"
    local section="$2"
    local interface_name="$3"

    local tag
    tag=$(get_outbound_tag_by_section "$section")

    config=$(sing_box_cm_add_interface_outbound "$config" "$tag" "$interface_name")

    echo "$config"
}

sing_box_cf_proxy_domain() {
    local config="$1"
    local inbound="$2"
    local domain="$3"
    local outbound="$4"

    tag="$(gen_id)"
    config=$(sing_box_cm_add_route_rule "$config" "$tag" "$inbound" "$outbound")
    config=$(sing_box_cm_patch_route_rule "$config" "$tag" "domain" "$domain")

    echo "$config"
}

sing_box_cf_override_domain_port() {
    local config="$1"
    local domain="$2"
    local port="$3"

    tag="$(gen_id)"
    config=$(sing_box_cm_add_options_route_rule "$config" "$tag")
    config=$(sing_box_cm_patch_route_rule "$config" "$tag" "domain" "$domain")
    config=$(sing_box_cm_patch_route_rule "$config" "$tag" "override_port" "$port")

    echo "$config"
}

sing_box_cf_add_single_key_reject_rule() {
    local config="$1"
    local inbound="$2"
    local key="$3"
    local value="$4"

    tag="$(gen_id)"
    config=$(sing_box_cm_add_reject_route_rule "$config" "$tag" "$inbound")
    config=$(sing_box_cm_patch_route_rule "$config" "$tag" "$key" "$value")

    echo "$config"
}
