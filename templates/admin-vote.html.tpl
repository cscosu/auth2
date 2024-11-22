{{ template "base.html.tpl" . }}

{{ define "title" }}Admin Vote | Cyber Security Club @ Ohio State{{ end }}

{{ define "content" }}
<div class="card">
  <div class="card-title">Vote Editor</div>
  <div class="card-content">
		<div id="candidate-title">
			<p class="font-bold">Candidate Title</p>
			<input type="text" id="candidate-title-in" name="candidateTitle"> {{ .candidateTitle }} </div>

		</div>
		<div id="candidate-list">
			{{
				range.candidateList
			}}

			<li>
				<input type="radio" id="{{ . }}" name="election"/>
				<!-- TODO - this id is not good. Could have spaces or smth -->
				<label for="{{ . }}"> {{ . }} </label>
			</li>

			{{
				end
			}}
		</div>
		<button
      class="grow justify-center secondary-button"
    >
      Submit
    </button>
	</div>
</div>
{{ end }}
