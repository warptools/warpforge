// store dark-mode selection
const onLoad = () => {
	const toggleSwitch = document.querySelector(
		'#dark-mode-checkbox'
	)
	const currentTheme = localStorage.getItem('theme')

	if (currentTheme) {
		if (currentTheme === 'dark') {
			toggleSwitch.checked = true
		}
	} else {
		const darkModeRequested = matchMedia('(prefers-color-scheme: dark)')
		toggleSwitch.checked = darkModeRequested
	}

	function switchTheme (e) {
		if (e.target.checked) {
			localStorage.setItem('theme', 'dark')
		} else {
			localStorage.setItem('theme', 'light')
		}
	}

	toggleSwitch.addEventListener('change', switchTheme, false)
}

addEventListener('load', onLoad)
