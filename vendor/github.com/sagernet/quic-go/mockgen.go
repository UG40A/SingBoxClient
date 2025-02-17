package quic

//go:generate sh -c "./mockgen_private.sh quic mock_send_conn_test.go github.com/sagernet/quic-go sendConn"
//go:generate sh -c "./mockgen_private.sh quic mock_sender_test.go github.com/sagernet/quic-go sender"
//go:generate sh -c "./mockgen_private.sh quic mock_stream_internal_test.go github.com/sagernet/quic-go streamI"
//go:generate sh -c "./mockgen_private.sh quic mock_crypto_stream_test.go github.com/sagernet/quic-go cryptoStream"
//go:generate sh -c "./mockgen_private.sh quic mock_receive_stream_internal_test.go github.com/sagernet/quic-go receiveStreamI"
//go:generate sh -c "./mockgen_private.sh quic mock_send_stream_internal_test.go github.com/sagernet/quic-go sendStreamI"
//go:generate sh -c "./mockgen_private.sh quic mock_stream_sender_test.go github.com/sagernet/quic-go streamSender"
//go:generate sh -c "./mockgen_private.sh quic mock_stream_getter_test.go github.com/sagernet/quic-go streamGetter"
//go:generate sh -c "./mockgen_private.sh quic mock_crypto_data_handler_test.go github.com/sagernet/quic-go cryptoDataHandler"
//go:generate sh -c "./mockgen_private.sh quic mock_frame_source_test.go github.com/sagernet/quic-go frameSource"
//go:generate sh -c "./mockgen_private.sh quic mock_ack_frame_source_test.go github.com/sagernet/quic-go ackFrameSource"
//go:generate sh -c "./mockgen_private.sh quic mock_stream_manager_test.go github.com/sagernet/quic-go streamManager"
//go:generate sh -c "./mockgen_private.sh quic mock_sealing_manager_test.go github.com/sagernet/quic-go sealingManager"
//go:generate sh -c "./mockgen_private.sh quic mock_unpacker_test.go github.com/sagernet/quic-go unpacker"
//go:generate sh -c "./mockgen_private.sh quic mock_packer_test.go github.com/sagernet/quic-go packer"
//go:generate sh -c "./mockgen_private.sh quic mock_mtu_discoverer_test.go github.com/sagernet/quic-go mtuDiscoverer"
//go:generate sh -c "./mockgen_private.sh quic mock_conn_runner_test.go github.com/sagernet/quic-go connRunner"
//go:generate sh -c "./mockgen_private.sh quic mock_quic_conn_test.go github.com/sagernet/quic-go quicConn"
//go:generate sh -c "./mockgen_private.sh quic mock_packet_handler_test.go github.com/sagernet/quic-go packetHandler"
//go:generate sh -c "./mockgen_private.sh quic mock_unknown_packet_handler_test.go github.com/sagernet/quic-go unknownPacketHandler"
//go:generate sh -c "./mockgen_private.sh quic mock_packet_handler_manager_test.go github.com/sagernet/quic-go packetHandlerManager"
//go:generate sh -c "./mockgen_private.sh quic mock_multiplexer_test.go github.com/sagernet/quic-go multiplexer"
//go:generate sh -c "./mockgen_private.sh quic mock_batch_conn_test.go github.com/sagernet/quic-go batchConn"
//go:generate sh -c "go run github.com/golang/mock/mockgen -package quic -self_package github.com/sagernet/quic-go -destination mock_token_store_test.go github.com/sagernet/quic-go TokenStore"
//go:generate sh -c "go run github.com/golang/mock/mockgen -package quic -self_package github.com/sagernet/quic-go -destination mock_packetconn_test.go net PacketConn"
