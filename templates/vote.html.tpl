{{ template "base.html.tpl" . }}

{{ define "title" }}Vote | Cyber Security Club @ Ohio State{{ end }}

{{ define "content" }}

<div id="voting-form">
	{{ template "voting-form.html.tpl" . }}
</div>

{{ end }}
