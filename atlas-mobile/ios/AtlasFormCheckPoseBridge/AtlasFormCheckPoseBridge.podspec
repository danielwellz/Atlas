Pod::Spec.new do |spec|
  spec.name = 'AtlasFormCheckPoseBridge'
  spec.version = '1.0.0'
  spec.summary = 'React Native bridge for on-device form-check pose detection'
  spec.description = 'Native module that streams pose angle frames and returns local form-check summaries.'
  spec.homepage = 'https://atlas.local/mobile'
  spec.license = { :type => 'MIT' }
  spec.author = { 'Atlas Mobile' => 'mobile@atlas.local' }
  spec.platform = :ios, '15.1'
  spec.source = { :path => '.' }
  spec.source_files = 'ios/**/*.{h,m,mm}'
  spec.requires_arc = true

  spec.dependency 'React-Core'
end
