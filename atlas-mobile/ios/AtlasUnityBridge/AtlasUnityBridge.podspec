Pod::Spec.new do |spec|
  spec.name = 'AtlasUnityBridge'
  spec.version = '1.0.0'
  spec.summary = 'React Native bridge for Unity as a Library'
  spec.description = 'Native module that opens Unity full-screen and bridges messages between React Native and Unity.'
  spec.homepage = 'https://atlas.local/mobile'
  spec.license = { :type => 'MIT' }
  spec.author = { 'Atlas Mobile' => 'mobile@atlas.local' }
  spec.platform = :ios, '15.1'
  spec.source = { :path => '.' }
  spec.source_files = 'ios/**/*.{h,m,mm}'
  spec.requires_arc = true

  spec.dependency 'React-Core'

  unity_framework_path = File.expand_path('../../../atlas-unity/Builds/ios/UnityFramework.framework', __dir__)
  if File.exist?(unity_framework_path)
    spec.vendored_frameworks = unity_framework_path
  end
end
