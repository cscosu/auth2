<div class="inline-flex gap-2">
  {{ if or .IsDesc .IsAsc }}
  <a
    class="group-hover:opacity-100 opacity-0"
    hx-boost="true"
    href="{{ .DeleteUrl }}"
  >
    {{ template "delete.html.tpl" }}
  </a>
  {{ end }}

  {{ if .IsDesc }}
  {{ .OrderNum }}
  <a hx-boost="true" href="{{ .NextUrl }}">
    {{ template "descending.html.tpl" }}
  </a>
  {{ else if .IsAsc }}
  {{ .OrderNum }}
  <a hx-boost="true" href="{{ .NextUrl }}">
    {{ template "ascending.html.tpl" }}
  </a>
  {{ else }}
  <a
    class="group-hover:opacity-100 opacity-0 text-gray-600"
    hx-boost="true"
    href="{{ .AddUrl }}"
  >
    {{ template "sort.html.tpl" }}
  </a>
  {{ end }}
</div>
