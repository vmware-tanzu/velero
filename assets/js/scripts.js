var menu = document.querySelector(".icon--menu");
var navigation = document.querySelector(".navigation");

menu.addEventListener("click", function() {
  navigation.classList.toggle("navigation--open");
});
