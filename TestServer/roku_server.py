import http.server
import socketserver

PORT = 8080

FEED_XML = """<?xml version="1.0" encoding="UTF-8"?>
<feed>
  <category name="Movies">
    <item>
      <id>movie-001</id>
      <title>Roku Sample Movie</title>
      <shortDescription>A short description of the sample movie.</shortDescription>
      <longDescription>A longer, more detailed description of the movie for testing.</longDescription>
      <thumbnail>http://192.168.254.19:8080/icon.jpg</thumbnail>
      <releaseDate>2024-05-01</releaseDate>
      <content>
        <video>
          <url>http://192.168.254.19:8080/roku_video.mp4</url>
          <quality>HD</quality>
          <streamFormat>mp4</streamFormat>
          <duration>138</duration>
        </video>
      </content>
    </item>
    <item>
      <id>movie-002</id>
      <title>Second Sample Movie</title>
      <shortDescription>Another short description.</shortDescription>
      <longDescription>Another long description for our test feed content.</longDescription>
      <thumbnail>http://192.168.254.19:8080/icon.jpg</thumbnail>
      <releaseDate>2024-06-10</releaseDate>
      <content>
        <video>
          <url>http://192.168.254.19:8080/roku_video.mp4</url>
          <quality>HD</quality>
          <streamFormat>mp4</streamFormat>
          <duration>95</duration>
        </video>
      </content>
    </item>
  </category>
  <category name="Documentaries">
    <item>
      <id>doc-001</id>
      <title>Test Documentary</title>
      <shortDescription>This is a test doc.</shortDescription>
      <longDescription>This documentary explains testing for Roku XML feeds.</longDescription>
      <thumbnail>http://192.168.254.19:8080/icon.jpg</thumbnail>
      <releaseDate>2023-10-12</releaseDate>
      <content>
        <video>
          <url>http://192.168.254.19:8080/roku_video.mp4</url>
          <quality>HD</quality>
          <streamFormat>mp4</streamFormat>
          <duration>200</duration>
        </video>
      </content>
    </item>
  </category>
</feed>
"""

class ReusableTCPServer(socketserver.TCPServer):
    allow_reuse_address = True

class FeedRequestHandler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/" or self.path == "/feed.xml":
            self.send_response(200)
            self.send_header("Content-type", "application/xml")
            self.end_headers()
            self.wfile.write(FEED_XML.encode("utf-8"))
        else:
            super().do_GET()  # Serve static files (icon.jpg, roku_video.mp4, etc.)

if __name__ == "__main__":
    print("Place 'icon.jpg' and 'roku_video.mp4' in the directory you run this script from.")
    with ReusableTCPServer(("", PORT), FeedRequestHandler) as httpd:
        print(f"Serving Roku XML feed at http://192.168.254.19:{PORT}/feed.xml")
        print(f"Serving icon.jpg at http://192.168.254.19:{PORT}/icon.jpg")
        print(f"Serving roku_video.mp4 at http://192.168.254.19:{PORT}/roku_video.mp4")
        print("Press Ctrl+C to quit.")
        httpd.serve_forever()