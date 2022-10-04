def release_ports(binding_file_path):
    from ixnetwork_restpy import TestPlatform
    from google.protobuf import text_format
    import binding_pb2

    with open(binding_file_path, 'rb') as fp:
        binding = text_format.Parse(fp.read(), binding_pb2.Binding())
        for device in binding.ates:
            ixia = device.ixnetwork
            if ixia and ixia.target:
                platform = TestPlatform(ixia.target)
                if ixia.username and ixia.password:
                    platform.Authenticate(ixia.username, ixia.password)
                
                vport = platform.Sessions.find() \
                    .Ixnetwork.Vport.find()
                vport.ReleasePort()