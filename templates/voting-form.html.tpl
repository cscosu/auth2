
<div class="card">
	{{ if not .hasVoted }}
	<div class="card-title">Cast your vote for {{ .candidateTitle }}</div>
	<div class="card-content">
	  <ul>
		{{ range .candidateList }}
		<li>
		  <input type="radio" id="{{ . }}" name="election"/>
		  <label for="{{ . }}"> {{ . }} </label>
		</li>
		{{ end }}
	  </ul>
	  <button 
		hx-post="/vote"
		hx-target="#voting-form"
		class="grow justify-center secondary-button"
	  >
		Submit
	  </button>
	</div>
	<div>
	  <canvas id="votePieChart"></canvas>
	</div>
	{{ else }}
	<p class="card-title">Thanks for voting!</p>
	{{ end }}
  </div>
  
  <script>
	document.addEventListener('DOMContentLoaded', function() {
	  // Predefined colors
	  const colors = [
		'#FF6384', '#36A2EB', '#FFCE56', '#4BC0C0', '#9966FF',
		'#FF9F40', '#FFCD56', '#C9CBCF', '#8D65D9', '#00A775'
	  ];
  
	  // Candidate list (from server-side JSON)
	  const candidateList = JSON.parse('{{ .candidateListJson }}'); // Now as a valid JSON string
  
	  // Generate chart data
	  const data = {
		labels: candidateList,
		datasets: [{
		  data: Array(candidateList.length).fill(1), // Initial equal votes for each candidate
		  backgroundColor: colors.slice(0, candidateList.length) // Assign colors dynamically
		}]
	  };
  
	  // Chart config
	  const config = {
		type: 'pie',
		data: data,
		options: {
		  responsive: true,
		  plugins: {
			legend: {
			  position: 'top',
			},
			title: {
			  display: true,
			  text: 'Vote Distribution'
			}
		  }
		}
	  };
  
	  // Render the chart
	  const votePieChart = new Chart(
		document.getElementById('votePieChart'),
		config
	  );
	});
  </script>