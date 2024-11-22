{{ template "admin.html.tpl" . }}

{{ define "title" }}Admin Users | Cyber Security Club @ Ohio State{{ end }}

{{ define "admin" }}
<div class="card">
  <div class="card-title">Users</div>
  <div class="card-content">
    <input
      type="search"
      name="search"
      placeholder="Search users..."
      hx-get="/admin/users"
      hx-trigger="input changed delay:500ms, search"
      hx-select="#users"
      hx-target="#users"
      hx-swap="outerHTML"
      hx-vals="{&quot;order&quot;: &quot;{{ .orderQuery }}&quot;}"
      value="{{ .searchQuery }}"
    />

    <div id="users">
      {{ $currentPage := .currentPage }}

      {{ $orderQueryParam := ""}}
      {{ if .orderQuery }}
      {{ $orderQueryParam = printf "&order=%s" .orderQuery }}
      {{ end }}

      {{ $searchQueryParam := ""}}
      {{ if .searchQuery }}
      {{ $searchQueryParam = printf "&search=%s" .searchQuery }}
      {{ end }}

      {{ range $page := .pageNumbers }}
      {{ if eq $page $currentPage }}
      <a
        href="/admin/users?page={{ $page }}{{ $orderQueryParam }}{{
          $searchQueryParam
        }}"
        hx-boost="true"
        class="px-2 py-1 rounded-sm bg-gray-100"
        >{{ $page }}</a
      >
      {{ else }}
      <a
        href="/admin/users?page={{ $page }}{{ $orderQueryParam }}{{
          $searchQueryParam
        }}"
        hx-boost="true"
        class="px-2 py-1 rounded-sm hover:bg-gray-100"
        >{{ $page }}</a
      >
      {{ end }}
      {{ end }}
      <div class="w-full overflow-x-scroll">
        <table class="table-auto">
          <thead>
            <tr class="border-b-2">
              <th class="px-4 py-2 text-left">
                <a
                  class="inline-flex"
                  target="_blank"
                  href="https://webauth.service.ohio-state.edu/~shibboleth/user-attribute-reference.html?article=employeenumber"
                  >Buck ID {{ template "key.html.tpl" }}</a
                >
              </th>
              <th class="px-4 py-2">
                <div class="inline-flex justify-between gap-6 group">
                  <a
                    target="_blank"
                    href="https://webauth.service.ohio-state.edu/~shibboleth/user-attribute-reference.html?article=mail"
                    >Name.#</a
                  >
                  {{ template "table-header.html.tpl" .orders.name_num }}
                </div>
              </th>
              <th class="px-4 py-2">
                <div class="inline-flex justify-between gap-6 group">
                  <span>Discord ID</span>
                  {{ template "table-header.html.tpl" .orders.discord_id }}
                </div>
              </th>
              <th class="px-4 py-2">
                <div class="inline-flex justify-between gap-6 group">
                  <a
                    target="_blank"
                    href="https://webauth.service.ohio-state.edu/~shibboleth/user-attribute-reference.html?article=preferred-names"
                    >Name</a
                  >
                  {{ template "table-header.html.tpl" .orders.display_name }}
                </div>
              </th>
              <th class="px-4 py-2">
                <div class="inline-flex justify-between gap-6 group">
                  <span>Last Seen</span>
                  {{ template "table-header.html.tpl" .orders.last_seen_timestamp }}
                </div>
              </th>
              <th class="px-4 py-2">
                <div class="inline-flex justify-between gap-6 group">
                  <span>Last Attended</span>
                  {{ template "table-header.html.tpl" .orders.last_attended_timestamp }}
                </div>
              </th>
              <th class="px-4 py-2">
                <div class="inline-flex justify-between gap-6 group">
                  <span>On Mailinglist</span>
                  {{ template "table-header.html.tpl" .orders.added_to_mailinglist }}
                </div>
              </th>
              <th class="px-4 py-2">
                <div class="inline-flex justify-between gap-6 group">
                  <span>Student</span>
                  {{ template "table-header.html.tpl" .orders.student }}
                </div>
              </th>
              <th class="px-4 py-2">
                <div class="inline-flex justify-between gap-6 group">
                  <span>Alum</span>
                  {{ template "table-header.html.tpl" .orders.alum }}
                </div>
              </th>
              <th class="px-4 py-2">
                <div class="inline-flex justify-between gap-6 group">
                  <span>Employee</span>
                  {{ template "table-header.html.tpl" .orders.employee }}
                </div>
              </th>
              <th class="px-4 py-2">
                <div class="inline-flex justify-between gap-6 group">
                  <span>Faculty</span>
                  {{ template "table-header.html.tpl" .orders.faculty }}
                </div>
              </th>
            </tr>
          </thead>
          <tbody
            class="[&_td]:px-4 [&_td]:py-2 [&_tr:not(:last-child)]:border-b"
          >
            {{
              range.users
            }}
            <tr>
              <td>{{ .BuckID }}</td>
              <td>{{ .NameNum }}</td>
              <td>{{ if not (eq .DiscordID 0) }}{{ .DiscordID }}{{ end }}</td>
              <td>{{ .DisplayName }}</td>
              <td>{{ .LastSeenTime }}</td>
              <td>
                {{ if .LastAttendedTime }}
                {{ .LastAttendedTime }}
                {{ end }}
              </td>
              <td>
                {{ if .AddedToMailingList }}
                {{ template "checkmark.html.tpl" }}
                {{ else }}
                {{ template "x.html.tpl" }}
                {{ end }}
              </td>
              <td>
                {{ if .Student }}
                {{ template "checkmark.html.tpl" }}
                {{ else }}
                {{ template "x.html.tpl" }}
                {{ end }}
              </td>
              <td>
                {{ if .Alum }}
                {{ template "checkmark.html.tpl" }}
                {{ else }}
                {{ template "x.html.tpl" }}
                {{ end }}
              </td>
              <td>
                {{ if .Employee }}
                {{ template "checkmark.html.tpl" }}
                {{ else }}
                {{ template "x.html.tpl" }}
                {{ end }}
              </td>
              <td>
                {{ if .Faculty }}
                {{ template "checkmark.html.tpl" }}
                {{ else }}
                {{ template "x.html.tpl" }}
                {{ end }}
              </td>
            </tr>
            {{
              end
            }}
          </tbody>
        </table>
      </div>
    </div>
  </div>
</div>
{{ end }}
