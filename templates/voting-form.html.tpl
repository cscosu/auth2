<div class="card">
	{{ if not .hasVoted }}
	<div class="card-title">Cast your vote for {{ .candidateTitle }}</div>
	<div class="card-content">
		<ul>
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
		</ul>
		<button 
			hx-post="/vote"
			hx-target="#voting-form"
			class="grow justify-center secondary-button"
		>
			Submit
		</button>
	</div>
	{{ else }}
	<p class="card-title">Thanks for voting!</p>
	{{ end }}
</div>
