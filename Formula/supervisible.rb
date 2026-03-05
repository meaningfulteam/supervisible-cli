class Supervisible < Formula
  desc "Supervisible command-line interface"
  homepage "https://supervisible.com"
  url "https://github.com/supervisible/supervisible-cli/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "REPLACE_WITH_RELEASE_SHA256"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/supervisible"
  end

  test do
    assert_match "supervisible", shell_output("#{bin}/supervisible --help")
  end
end
