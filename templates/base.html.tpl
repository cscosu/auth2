<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="title" content="{{ block `title` . }}{{ end }}" />
    <script src="https://unpkg.com/htmx.org@2.0.3"></script>
    <link href="/static/tailwind.css" rel="stylesheet" />
    <title>{{ block "title" . }}{{ end }}</title>
  </head>
  <body class="min-h-screen flex flex-col">
    <header class="border-b flex justify-center py-2">
      <div class="container mx-2">
        <nav class="flex items-center justify-between">
          <div class="flex items-center">
            <a
              hx-boost="true"
              href="/"
              class="mr-6 flex items-center space-x-2"
            >
              <img src="https://osucyber.club/img/logo.png" width="28" />
            </a>
            <a hx-boost="true" href="/">OSU Cyber Security Club Auth</a>
          </div>
          {{ if .nameNum }}
          <div>
            <span class="mr-4">Signed in as {{ .nameNum }}</span>
            <a href="/signout" class="primary-button">Sign out</a>
          </div>
          {{ else }}
          <a href="/signin" class="primary-button">Sign in with OSU</a>
          {{ end }}
        </nav>
      </div>
    </header>
    <div class="flex flex-1 justify-center">
      <div class="container mx-2 mt-6">{{ block "content" . }}{{ end }}</div>
    </div>
    <footer class="flex justify-center">
      <div class="container py-6 mx-2">
        <a href="https://osucyber.club"
          >Cyber Security Club @ The Ohio State University</a
        >
      </div>
    </footer>
  </body>
</html>
